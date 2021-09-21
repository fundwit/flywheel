package tracing

import (
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestTracingIngress(t *testing.T) {
	RegisterTestingT(t)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	router := gin.Default()
	router.Use(TracingIngress())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("new root trace", func(t *testing.T) {
		tracer.Reset()

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		status, _, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))

		spans := tracer.FinishedSpans()
		Expect(len(spans)).To(Equal(1))
		s := spans[0]
		Expect(s.OperationName).To(Equal("GET /test"))
		Expect(s.ParentID).To(Equal(0))
		Expect(time.Since(s.StartTime) < time.Second).To(BeTrue())
		Expect(time.Since(s.FinishTime) < time.Second).To(BeTrue())
		Expect(s.SpanContext.SpanID).ToNot(BeZero())
		Expect(s.SpanContext.TraceID).To(BeZero())
		Expect(s.SpanContext.Sampled).To(BeFalse())
	})

	t.Run("child trace", func(t *testing.T) {
		tracer.Reset()

		clientSpan := tracer.StartSpan("client")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
		status, _, _ := testinfra.ExecuteRequest(req, router)

		clientSpan.Finish()

		Expect(status).To(Equal(http.StatusOK))

		spans := tracer.FinishedSpans()
		Expect(len(spans)).To(Equal(2))
		s0 := spans[1]
		Expect(s0.OperationName).To(Equal("client"))
		Expect(s0.ParentID).To(BeZero())
		Expect(s0.SpanContext.SpanID).ToNot(BeZero())
		Expect(s0.SpanContext.TraceID).ToNot(BeZero())
		Expect(s0.SpanContext.Sampled).To(BeTrue())

		s1 := spans[0]
		Expect(s1.OperationName).To(Equal("GET /test"))
		Expect(s1.ParentID).To(Equal(s0.SpanContext.SpanID))
		Expect(s1.SpanContext.SpanID).ToNot(BeZero())
		Expect(s1.SpanContext.TraceID).To(Equal(s1.SpanContext.TraceID))
		Expect(s1.SpanContext.Sampled).To(BeTrue())
	})
}
