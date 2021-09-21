package tracing

import (
	"io"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegerconfig "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	jaegermetrics "github.com/uber/jaeger-lib/metrics"
)

func NewTracer() (opentracing.Tracer, io.Closer, error) {
	// Sample configuration for testing. Use constant sampling to sample every
	// trace and enable LogSpan to log every span via configured Logger.
	cfg := &jaegerconfig.Configuration{
		ServiceName: "flywheel",
		Sampler: &jaegerconfig.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegerconfig.ReporterConfig{
			LogSpans:          true,
			CollectorEndpoint: os.Getenv("JAEGER_ENDPOINT"),
		},
	}

	// Initialize tracer with a logger and a metrics factory
	// the returned closer func can be used to flush buffers before shutdown
	tracer, closer, err := cfg.NewTracer(
		jaegerconfig.Logger(jaegerlog.StdLogger),
		jaegerconfig.Metrics(jaegermetrics.NullFactory),
	)
	return tracer, closer, err
}
