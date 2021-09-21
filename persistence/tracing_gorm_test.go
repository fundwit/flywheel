package persistence_test

import (
	"context"
	"flywheel/domain"
	"flywheel/testinfra"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	otgorm "github.com/smacker/opentracing-gorm"
)

func TestGormTracing(t *testing.T) {
	RegisterTestingT(t)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
	var testDatabase *testinfra.TestDatabase

	t.Run("gorm tracing should be ignored when parent span not found", func(t *testing.T) {
		defer gormTracingTestTeardown(t, testDatabase)
		gormTractingTestSetup(t, &testDatabase)

		tracer.Reset()

		// case1
		db := testDatabase.DS.GormDB()
		r := []domain.Project{}
		Expect(db.Find(&r).Error).To(BeNil())
		Expect(len(r)).To(BeZero())

		spans := tracer.FinishedSpans()
		Expect(len(spans)).To(Equal(0))

		// case2
		r = []domain.Project{}
		Expect(otgorm.SetSpanToGorm(context.Background(), db).Find(&r).Error).To(BeNil())
		Expect(len(r)).To(BeZero())
		spans = tracer.FinishedSpans()
		Expect(len(spans)).To(Equal(0))
	})

	t.Run("gorm tracing should be work with parent span", func(t *testing.T) {
		defer gormTracingTestTeardown(t, testDatabase)
		gormTractingTestSetup(t, &testDatabase)

		tracer.Reset()

		clientSpan := tracer.StartSpan("client")
		// inject span into context
		ctx := opentracing.ContextWithSpan(context.Background(), clientSpan)

		// extract span from context and then inject into db.values.
		db := otgorm.SetSpanToGorm(ctx, testDatabase.DS.GormDB())

		r := []domain.Project{}
		Expect(db.Find(&r).Error).To(BeNil())
		Expect(len(r)).To(BeZero())

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
		Expect(s1.OperationName).To(Equal("sql"))
		Expect(s1.ParentID).To(Equal(s0.SpanContext.SpanID))
		Expect(s1.SpanContext.SpanID).ToNot(BeZero())
		Expect(s1.SpanContext.TraceID).To(Equal(s1.SpanContext.TraceID))
		Expect(s1.SpanContext.Sampled).To(BeTrue())
	})
}

func gormTractingTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) {
	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	Expect(db.DS.GormDB().AutoMigrate(&domain.Project{}).Error).To(BeNil())
}

func gormTracingTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}
