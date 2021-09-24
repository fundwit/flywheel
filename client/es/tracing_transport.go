package es

import (
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

type TracingTransport struct {
	Transport http.RoundTripper
}

func (t *TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Context() != nil {
		parentSpan := opentracing.SpanFromContext(req.Context())
		if parentSpan != nil {
			tracer := parentSpan.Tracer()
			childSpan := tracer.StartSpan(req.Method+" "+req.RequestURI, opentracing.ChildOf(parentSpan.Context()))
			defer childSpan.Finish()

			ext.SpanKindRPCClient.Set(childSpan)
			ext.HTTPUrl.Set(childSpan, req.URL.String())
			ext.HTTPMethod.Set(childSpan, req.Method)

			// Inject the client span context into the headers
			tracer.Inject(childSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
			res, err := t.Transport.RoundTrip(req)

			ext.HTTPStatusCode.Set(childSpan, uint16(res.StatusCode))
			ext.Error.Set(childSpan, res.StatusCode >= 400)

			return res, err
		}
	}

	return t.Transport.RoundTrip(req)
}
