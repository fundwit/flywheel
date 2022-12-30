package tracing

import "github.com/opentracing/opentracing-go/ext"

var (
	ErrorDetail = ext.StringTagName("error.detail")
)
