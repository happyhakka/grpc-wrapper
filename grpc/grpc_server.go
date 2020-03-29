package grpc

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"runtime"
	"strings"

	_ "net/http/pprof"

	. "github.com/happyhakka/grpc-wrapper/log"
	"github.com/happyhakka/grpc-wrapper/trc"

	"go.uber.org/zap"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type GrpcServeWrapper struct {
	svr *grpc.Server
	opt *GrpcSysOption
}

func NewGrpcServeWrapper() *GrpcServeWrapper {
	p := &GrpcServeWrapper{}
	p.opt = NewGrpcSysOption()
	return p
}

func (p *GrpcServeWrapper) SetOption(o *GrpcSysOption) {
	p.opt = o
}

func (p *GrpcServeWrapper) Init(serviceName string, serviceAddr string) {
	p.opt.ServiceName = serviceName
	p.opt.ServiceAddr = serviceAddr
	if !strings.Contains(serviceAddr, ":") {
		p.opt.ServiceAddr = ":" + serviceAddr
	}

	fmt.Printf("grpc-server-option: %#v\n", p.opt)

	//设置grpc拦截器
	interceptors := make([]grpc.UnaryServerInterceptor, 0)
	streamInterceptors := make([]grpc.StreamServerInterceptor, 0)

	streamInterceptors = append(streamInterceptors, grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)))
	interceptors = append(interceptors, grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)))

	if p.opt.TracerFlag {
		//tracer初始化
		tracer, err := trc.InitTracer(p.opt.ServiceName, p.opt.TracerAddr)
		if err != nil {
			grpclog.Errorf("init open tracing fail! error<%v>\n", err)
			return
		}

		streamInterceptors = append(streamInterceptors, grpc_opentracing.StreamServerInterceptor(grpc_opentracing.WithTracer(tracer)))
		interceptors = append(interceptors, grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer)))
	}

	//日志初始化,设置GRPC日志
	if p.opt.LogFlag {
		logger, err := InitLogger(p.opt.LogFile)
		if err != nil || logger == nil {
			grpclog.Error("init logger fail! error<%v>\n", err)
			return
		}

		//设置grpc日志
		grpc_zap.ReplaceGrpcLoggerV2(logger)
		streamInterceptors = append(streamInterceptors, grpc_zap.StreamServerInterceptor(logger))
		interceptors = append(interceptors, grpc_zap.UnaryServerInterceptor(logger))
	}

	if p.opt.PromFlag {
		streamInterceptors = append(streamInterceptors, grpc_prometheus.StreamServerInterceptor)
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)
	}

	streamInterceptors = append(streamInterceptors, grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(panicHandler)))
	interceptors = append(interceptors, grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(panicHandler)))

	//服务注册与注销
	//if p.opt.RegFlag {
	//	api.Register(etcdAddr, scheme, serviceName, serviceAddr, ttl)
	//	defer api.UnRegister()
	//}

	//grpc拦截器设置
	p.svr = grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...)),
		grpc.MaxRecvMsgSize(math.MaxInt32))

}

func (p *GrpcServeWrapper) GetServer() *grpc.Server {
	return p.svr
}

func (p *GrpcServeWrapper) UnInit() {

}

func (p *GrpcServeWrapper) Run() {
	startMetrics(p.svr, p.opt.PromAddr)
	listen, err := net.Listen("tcp", p.opt.ServiceAddr)
	if err != nil {
		grpclog.Errorf("grpc listend failed! service-addr:%v, error:<%v>", p.opt.ServiceAddr, err)
		panic(err.Error())
	}
	grpclog.Infof("grpc-service: %v listen: %v", p.opt.ServiceName, p.opt.ServiceAddr)
	reflection.Register(p.svr)
	err = p.svr.Serve(listen)
	if err != nil {
		grpclog.Errorf("grpc startup failed!  service-addr:%v, error:<%v>", p.opt.ServiceAddr, err)
		return
	}
}

var panicHandler = grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	grpclog.Errorf("grpc panice recovered", zap.String("panic_recovered", string(buf)))
	return status.Errorf(codes.Internal, "%s", p)
})

func startMetrics(grpcServer *grpc.Server, promAddr string) {
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(grpcServer)
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		grpclog.Infof("prometheus listen: %v/metris", promAddr)
		if err := http.ListenAndServe(promAddr, nil); err != nil {
			grpclog.Errorf("prometheus listen failed! bind-addr:%v, error:<%v>", promAddr, err)
			panic(err)
		}
	}()
}
