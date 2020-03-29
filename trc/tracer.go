package trc

import (
	"fmt"
	"io"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var (
	Tracer opentracing.Tracer
	Closer io.Closer
)

// newTracer 创建一个jaeger Tracer
func newTracer(serviceName string, addr string) (opentracing.Tracer, io.Closer, error) {

	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
		},
	}

	sender, err := jaeger.NewUDPTransport(addr, 0)
	if err != nil {
		return nil, nil, err
	}

	reporter := jaeger.NewRemoteReporter(sender)
	tracer, closer, err := cfg.NewTracer(jaegercfg.Reporter(reporter))
	return tracer, closer, err
}

func InitTracer(serviceName string, jaegerAddr string) (opentracing.Tracer, error) {
	var err error
	Tracer, Closer, err = newTracer(serviceName, jaegerAddr)
	if err != nil {
		fmt.Printf("create tracer fail! error<%v>\n", err)
		return nil, err
	}
	opentracing.SetGlobalTracer(Tracer)
	return Tracer, nil
}

func UnInit() {
	if Closer != nil {
		Closer.Close()
	}
}
