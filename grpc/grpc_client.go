package grpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/happyhakka/grpc-wrapper/trc"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

//GrpcPool pool info
type GrpcPool struct {
	mu          sync.Mutex
	idleTimeout time.Duration
	conns       chan *grpcIdleConn
	factory     func() (*grpc.ClientConn, error)
	close       func(*grpc.ClientConn) error
}

type grpcIdleConn struct {
	conn *grpc.ClientConn
	t    time.Time
}

//Get get from pool
func (c *GrpcPool) Get() (*grpc.ClientConn, error) {
	c.mu.Lock()
	conns := c.conns
	c.mu.Unlock()

	if conns == nil {
		return nil, errClosed
	}

	for {
		select {
		case wrapConn := <-conns:
			if wrapConn == nil {
				return nil, errClosed
			}
			//判断是否超时，超时则丢弃
			if timeout := c.idleTimeout; timeout > 0 {
				if wrapConn.t.Add(timeout).Before(time.Now()) {
					//丢弃并关闭该链接
					c.close(wrapConn.conn)
					continue
				}
			}
			return wrapConn.conn, nil
		default:
			conn, err := c.factory()
			if err != nil {
				return nil, err
			}

			return conn, nil
		}
	}
}

//Put put back to pool
func (c *GrpcPool) Put(conn *grpc.ClientConn) error {
	if conn == nil {
		return errRejected
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conns == nil {
		return c.close(conn)
	}

	select {
	case c.conns <- &grpcIdleConn{conn: conn, t: time.Now()}:
		return nil
	default:
		//连接池已满，直接关闭该链接
		return c.close(conn)
	}
}

//Close close pool
func (c *GrpcPool) Close() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.factory = nil
	closeFun := c.close
	c.close = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)
	for wrapConn := range conns {
		closeFun(wrapConn.conn)
	}
}

//IdleCount idle connection count
func (c *GrpcPool) idleCount() int {
	c.mu.Lock()
	conns := c.conns
	c.mu.Unlock()
	return len(conns)
}

var (
	retriableErrors = []codes.Code{codes.Unavailable, codes.DataLoss}
	retryTimeout    = 30
	retryTimes      = 3
)

func getDefualtDialOption(o *PoolOption) []grpc.DialOption {
	opts := make([]grpc.DialOption, 0)
	streamOpts := make([]grpc.DialOption, 0)

	opts = append(opts, grpc.WithInsecure())

	if o.PromFlag == false && strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "on" || strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "true" {
		o.PromFlag = true
	} else if strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "off" || strings.ToLower(os.Getenv(ENV_PROM_FLAG)) == "false" {
		o.PromFlag = false
	}

	if o.PromFlag {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor))
		streamOpts = append(streamOpts, grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor))
	}

	if o.ClientRetryFlag == false && strings.ToLower(os.Getenv(ENV_CLT_RETRY_FLAG)) == "on" || strings.ToLower(os.Getenv(ENV_CLT_RETRY_FLAG)) == "true" {
		o.ClientRetryFlag = true
	} else if strings.ToLower(os.Getenv(ENV_CLT_RETRY_FLAG)) == "off" || strings.ToLower(os.Getenv(ENV_CLT_RETRY_FLAG)) == "false" {
		o.ClientRetryFlag = false
	}

	if o.ClientRetryFlag {
		if o.ClientRetryTimes <= 0 && len(os.Getenv(ENV_CLT_RETRY_TIMES)) > 0 {
			times, _ := strconv.Atoi(os.Getenv(ENV_CLT_RETRY_TIMES))
			o.ClientRetryTimes = uint(times)
		}

		if o.ClientRetryTimeout <= 0 && len(os.Getenv(ENV_CLT_RETRY_TIMEOUT)) > 0 {
			to, _ := strconv.Atoi(os.Getenv(ENV_CLT_RETRY_TIMEOUT))
			o.ClientRetryTimeout = int32(to)
		}

		if o.ClientRetryTimes <= 0 {
			o.ClientRetryTimes = uint(retryTimes)
		}

		if o.ClientRetryTimeout <= 0 {
			o.ClientRetryTimeout = int32(retryTimeout)
		}

		rto := time.Duration(time.Duration(o.ClientRetryTimeout) * time.Second)
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithCodes(retriableErrors...), grpc_retry.WithMax(o.ClientRetryTimes), grpc_retry.WithBackoff(grpc_retry.BackoffLinear(rto)))))
		streamOpts = append(streamOpts, grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(grpc_retry.WithCodes(retriableErrors...), grpc_retry.WithMax(o.ClientRetryTimes), grpc_retry.WithBackoff(grpc_retry.BackoffLinear(rto)))))
	}

	if o.TracerFlag == false && strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "on" || strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "true" {
		o.TracerFlag = true
	} else if strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "off" || strings.ToLower(os.Getenv(ENV_TRC_FLAG)) == "false" {
		o.TracerFlag = false
	}

	if o.TracerFlag {
		o.TracerAddr = os.Getenv(ENV_TRC_ADDR)
		if o.TracerAddr == "" {
			fmt.Println("tracer host addr no found!")
		} else {
			tracer, err := trc.InitTracer(o.ServiceName, o.TracerAddr)
			if err != nil {
				fmt.Printf("init open tracing fail! error<%v>\n", err)
			} else {
				opts = append(opts, grpc.WithUnaryInterceptor(grpc_opentracing.UnaryClientInterceptor(grpc_opentracing.WithTracer(tracer))))
				streamOpts = append(streamOpts, grpc.WithStreamInterceptor(grpc_opentracing.StreamClientInterceptor(grpc_opentracing.WithTracer(tracer))))
			}
		}
	}
	opts = append(opts, streamOpts...)
	opts = append(opts, grpc.WithBlock())
	return opts
}

//NewGrpcPool init grpc pool
func NewGrpcPool(o *PoolOption, dialOptions ...grpc.DialOption) (*GrpcPool, error) {
	fmt.Printf("grpc-pool-option:%#v\n", o)
	if err := o.validate(); err != nil {
		return nil, err
	}

	//init pool
	pool := &GrpcPool{
		conns: make(chan *grpcIdleConn, o.MaxCap),
		factory: func() (*grpc.ClientConn, error) {
			target := o.nextTarget()
			if target == "" {
				return nil, errTargets
			}

			//ctx, cancel := context.WithTimeout(context.Background(), o.DialTimeout)
			//defer cancel()
			//return grpc.DialContext(ctx, target, dialOptions...)
			return grpc.Dial(target, dialOptions...)
		},
		close:       func(v *grpc.ClientConn) error { return v.Close() },
		idleTimeout: o.IdleTimeout,
	}

	//danamic update targets
	o.update()

	//init make conns
	for i := 0; i < o.InitCap; i++ {
		conn, err := pool.factory()
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.conns <- &grpcIdleConn{conn: conn, t: time.Now()}
	}

	return pool, nil
}

//NewGrpcPoolDefault init grpc pool
func NewDefaultGrpcPool(o *PoolOption) (*GrpcPool, error) {
	opts := getDefualtDialOption(o)
	return NewGrpcPool(o, opts...)
}
