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
		serverSpan := tracer.StartSpan(ctx.Request.Method+" "+ctx.FullPath(), ext.RPCServerOption(spanCtx))
		ext.HTTPMethod.Set(serverSpan, ctx.Request.Method)
		defer serverSpan.Finish()

		ctx.Request = ctx.Request.WithContext(opentracing.ContextWithSpan(ctx.Request.Context(), serverSpan))
		ctx.Next()

		ext.HTTPStatusCode.Set(serverSpan, uint16(ctx.Writer.Status()))
		ext.Error.Set(serverSpan, ctx.Writer.Status() >= 400)
		ext.HTTPUrl.Set(serverSpan, ctx.Request.RequestURI)
	}
}
