package grpc

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ENV_LOG_FLAG        = "env_log_flag"
	ENV_LOG_CONFIG_FILE = "env_log_config_file"
	ENV_TRC_FLAG        = "env_trc_flag"
	ENV_TRC_ADDR        = "env_trc_addr"
	ENV_PROM_FLAG       = "env_prom_flag"
	ENV_PROM_ADDR       = "env_prom_addr"
	ENV_REG_FLAG        = "env_reg_flag"
	ENV_REG_ADDR        = "env_reg_addr"

	ENV_CLT_RETRY_FLAG    = "env_clt_retry_flag"    //是否开启客户端重启机制
	ENV_CLT_RETRY_TIMES   = "env_clt_retry_times"   //重试次数
	ENV_CLT_RETRY_TIMEOUT = "env_clt_retry_timeout" //重试超时
)

type GrpcSysOption struct {
	ServiceName string //服务名称ReportService
	ServiceAddr string //服务监听端口
	LogFlag     bool   //是否开启日志
	PromFlag    bool   //性能监控
	TracerFlag  bool   //是否开启分布式跟踪
	RegFlag     bool   //注册中心
	LogFile     string //日志配置文件
	PromAddr    string //性能监控端口
	TracerAddr  string //调用链服务地址
	RegAddr     string //注册中心地址
	AuthFlag    bool   //是否开启认证功能
}

func NewGrpcSysOption() *GrpcSysOption {
	p := &GrpcSysOption{}
	p.Init()
	return p
}

func (p *GrpcSysOption) Init() {
	p.LogFlag = true
	p.PromFlag = true

	if len(p.ServiceAddr) <= 0 {
		p.ServiceAddr = ":6066"
	}

	if p.LogFlag == false && (strings.ToLower(os.Getenv(ENV_LOG_FLAG)) == "on" || strings.ToLower(os.Getenv(ENV_LOG_FLAG)) == "true") {
		p.LogFlag = true
		p.LogFile = os.Getenv(ENV_LOG_CONFIG_FILE)
		if p.LogFile == "" {
			p.LogFile = "log.json"
		}
	} else {
		if (p.LogFlag == true) && p.LogFile == "" {
			if p.LogFile == "" {
				p.LogFile = "log.json"
			}
		}
	}

	if p.TracerFlag == false && (strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "on" || strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "true") {
		p.TracerFlag = true
		p.TracerAddr = os.Getenv(ENV_TRC_ADDR)
		if p.TracerAddr == "" {
			fmt.Println("tracer host addr no found!")
		}
	}

	if strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "off" || strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "false" {
		p.PromFlag = false
	} else {
		p.PromFlag = true
		p.PromAddr = os.Getenv(ENV_PROM_ADDR)
		if p.PromAddr == "" {
			p.PromAddr = ":5055"
		}
	}
}

var (
	errClosed   = errors.New("pool is closed")
	errInvalid  = errors.New("invalid config")
	errRejected = errors.New("connection is nil. rejecting")
	errTargets  = errors.New("targets server is empty")
)

func init() {
	rand.NewSource(time.Now().UnixNano())
}

//Options pool options
type PoolOption struct {
	lock sync.RWMutex
	//targets node
	targets *[]string

	//targets channel
	input chan *[]string

	//InitTargets init targets
	InitTargets []string
	// init connection
	InitCap int
	// max connections
	MaxCap       int
	DialTimeout  time.Duration
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	ClientRetryFlag    bool   //是否开启客户端重启机制
	ClientRetryTimes   uint   //重试次数
	ClientRetryTimeout int32  //重试超时
	PromFlag           bool   //性能监控拦截器
	TracerFlag         bool   //是否开启分布式跟踪
	ServiceName        string //服务名称，用于分布式跟踪
	TracerAddr         string //分布式跟踪地址
}

// Input is the input channel
func (o *PoolOption) Input() chan<- *[]string {
	return o.input
}

// update targets
func (o *PoolOption) update() {
	//init targets
	o.targets = &o.InitTargets

	go func() {
		for targets := range o.input {
			if targets == nil {
				continue
			}

			o.lock.Lock()
			o.targets = targets
			o.lock.Unlock()
		}
	}()

}

// NewPoolOptions returns a new NewPoolOptions instance with sane defaults.
func NewPoolOption(serviceName string, serviceAddrs []string, minSize int, maxSize int) *PoolOption {
	o := &PoolOption{}
	o.ServiceName = serviceName
	o.InitTargets = serviceAddrs

	if minSize <= 0 {
		minSize = 5
	}

	if maxSize <= 5 {
		maxSize = 100
	}
	o.InitCap = minSize
	o.MaxCap = maxSize

	o.DialTimeout = 5 * time.Second
	o.ReadTimeout = 5 * time.Second
	o.WriteTimeout = 5 * time.Second
	o.IdleTimeout = 60 * time.Second
	return o
}

// validate checks a Config instance.
func (o *PoolOption) validate() error {
	if o.InitTargets == nil ||
		o.InitCap <= 0 ||
		o.MaxCap <= 0 ||
		o.InitCap > o.MaxCap ||
		o.DialTimeout == 0 ||
		o.ReadTimeout == 0 ||
		o.WriteTimeout == 0 {
		return errInvalid
	}
	return nil
}

//nextTarget next target implement load balance
func (o *PoolOption) nextTarget() string {
	o.lock.RLock()
	defer o.lock.RUnlock()

	tlen := len(*o.targets)
	if tlen <= 0 {
		return ""
	}

	//rand server
	return (*o.targets)[rand.Int()%tlen]
}
