package es

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
)

type AlwaysFailedTransport struct {
}

func (t *AlwaysFailedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("mock error")
}

func TestTracingTransport(t *testing.T) {
	RegisterTestingT(t)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts1.Close()

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
		Expect(s1.Tags()).To(Equal(map[string]interface{}{
			"span.kind":        ext.SpanKindEnum("client"),
			"http.url":         ts.URL,
			"http.method":      "GET",
			"http.status_code": uint16(200),
			"error":            false,
		}))
	})

	t.Run("child trace with error", func(t *testing.T) {
		tracer.Reset()

		client := &http.Client{Transport: &TracingTransport{Transport: http.DefaultTransport}}
		req, err := http.NewRequest("GET", ts1.URL, nil)
		Expect(err).To(BeNil())

		clientSpan := tracer.StartSpan("client")
		req = req.WithContext(opentracing.ContextWithSpan(context.Background(), clientSpan))

		res, err := client.Do(req)
		Expect(err).To(BeNil())

		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
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
		Expect(s1.Tags()).To(Equal(map[string]interface{}{
			"span.kind":        ext.SpanKindEnum("client"),
			"http.url":         ts1.URL,
			"http.method":      "GET",
			"http.status_code": uint16(400),
			"error":            true,
		}))
	})

	t.Run("child trace with no-response error", func(t *testing.T) {
		tracer.Reset()

		client := &http.Client{Transport: &TracingTransport{Transport: &AlwaysFailedTransport{}}}
		req, err := http.NewRequest("GET", "http://127.0.0.1:12345", nil)
		Expect(err).To(BeNil())

		clientSpan := tracer.StartSpan("client")
		req = req.WithContext(opentracing.ContextWithSpan(context.Background(), clientSpan))

		res, err := client.Do(req)
		Expect(res).To(BeNil())
		var urlErr *url.Error
		Expect(errors.As(err, &urlErr)).To(BeTrue())
		Expect(urlErr.Op).To(Equal("Get"))
		Expect(urlErr.URL).To(Equal("http://127.0.0.1:12345"))
		Expect(urlErr.Err.Error()).To(Equal("mock error"))
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
		Expect(s1.Tags()).To(Equal(map[string]interface{}{
			"span.kind":    ext.SpanKindEnum("client"),
			"http.url":     "http://127.0.0.1:12345",
			"http.method":  "GET",
			"error":        true,
			"error.detail": "mock error",
		}))
	})
}
