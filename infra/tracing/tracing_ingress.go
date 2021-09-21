package tracing

import (
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

func TracingIngress() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tracer := opentracing.GlobalTracer()
		spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(ctx.Request.Header))
		serverSpan := tracer.StartSpan(ctx.Request.Method+" "+ctx.Request.RequestURI, ext.RPCServerOption(spanCtx))
		defer serverSpan.Finish()

		ctx.Request = ctx.Request.WithContext(opentracing.ContextWithSpan(ctx.Request.Context(), serverSpan))

		ctx.Next()
	}
}
