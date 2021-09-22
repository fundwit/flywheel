package es

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestTracingTransport(t *testing.T) {
	RegisterTestingT(t)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	t.Run("no context", func(t *testing.T) {
		tracer.Reset()

		client := &http.Client{Transport: &TracingTransport{Transport: http.DefaultTransport}}
		req, err := http.NewRequest("GET", ts.URL, nil)
		Expect(err).To(BeNil())
		res, err := client.Do(req)
		Expect(err).To(BeNil())

		Expect(res.StatusCode).To(Equal(http.StatusOK))
		Expect(err).To(BeNil())

		Expect(len(tracer.FinishedSpans())).To(BeZero())
	})

	t.Run("child trace", func(t *testing.T) {
		tracer.Reset()

		client := &http.Client{Transport: &TracingTransport{Transport: http.DefaultTransport}}
		req, err := http.NewRequest("GET", ts.URL, nil)
		Expect(err).To(BeNil())

		clientSpan := tracer.StartSpan("client")
		req = req.WithContext(opentracing.ContextWithSpan(context.Background(), clientSpan))

		res, err := client.Do(req)
		Expect(err).To(BeNil())

		Expect(res.StatusCode).To(Equal(http.StatusOK))
		Expect(err).To(BeNil())
		clientSpan.Finish()

		spans := tracer.FinishedSpans()
		Expect(len(spans)).To(Equal(2))

		s0 := spans[1]
		Expect(s0.OperationName).To(Equal("client"))
		Expect(s0.ParentID).To(BeZero())
		Expect(s0.SpanContext.SpanID).ToNot(BeZero())
		Expect(s0.SpanContext.TraceID).ToNot(BeZero())
		Expect(s0.SpanContext.Sampled).To(BeTrue())

		s1 := spans[0]
		Expect(s1.OperationName).To(Equal("GET "))
		Expect(s1.ParentID).To(Equal(s0.SpanContext.SpanID))
		Expect(s1.SpanContext.SpanID).ToNot(BeZero())
		Expect(s1.SpanContext.TraceID).To(Equal(s1.SpanContext.TraceID))
		Expect(s1.SpanContext.Sampled).To(BeTrue())
	})

}
