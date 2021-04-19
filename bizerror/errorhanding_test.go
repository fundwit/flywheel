package bizerror_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/i18n"
	"flywheel/testinfra"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("ErrorHandlingMiddleware", func() {
	var r *gin.Engine
	BeforeEach(func() {
		r = gin.Default()
		r.Use(bizerror.ErrorHandling())
	})

	Context("panic handling", func() {
		It("should be able to handle panic with error", func() {
			r.GET("/", func(c *gin.Context) { panic(fmt.Errorf("some error")) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"` + i18n.CommonInternalServerError + `", "message":"some error", "data": null}`))
		})

		It("should be able to handle panic with other object", func() {
			r.GET("/", func(c *gin.Context) { panic("some error") })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"` + i18n.CommonInternalServerError + `", "message":"some error", "data": null}`))
		})

		It("should be able to handle panic with biz error", func() {
			r.GET("/", func(c *gin.Context) {
				panic(&demoError{Message: "some message in demo error", Data: 1234})
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(444))
			Expect(body).To(MatchJSON(`{"code":"common.demo", "message":"demo error: some message in demo error", "data": 1234}`))
		})

		It("should not be able to handle panic with nil", func() {
			r.GET("/", func(c *gin.Context) { panic(nil) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(Equal(""))
		})
	})

	Context("gin.Error handling", func() {
		It("should be able to handle error in gin.Context.Errors", func() {
			r.GET("/", func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{Err: errors.New("error1")})
				c.Errors = append(c.Errors, &gin.Error{Err: errors.New("error2")})
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"` + i18n.CommonInternalServerError + `", "message":"error2", "data": null}`))
		})

		It("should be able to handle panic error first even gin.Context.Errors is not empty", func() {
			r.GET("/", func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{Err: errors.New("error1")})
				panic("panic error")
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"` + i18n.CommonInternalServerError + `", "message":"panic error", "data": null}`))
		})

		It("should handle gin.Context.Errors when panic nil", func() {
			r.GET("/", func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{Err: errors.New("error1")})
				panic(nil)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"` + i18n.CommonInternalServerError + `", "message":"error1", "data": null}`))
		})
	})

	Context("specified errors", func() {
		It("should handle common.ErrForbidden", func() {
			r.GET("/", func(c *gin.Context) {
				_ = c.Error(bizerror.ErrForbidden)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusForbidden))
			Expect(body).To(MatchJSON(`{"code":"security.forbidden", "message":"access forbidden", "data": null}`))
		})
		It("should handle gorm.ErrRecordNotFound", func() {
			r.GET("/", func(c *gin.Context) {
				_ = c.Error(gorm.ErrRecordNotFound)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			status, body, _ := testinfra.ExecuteRequest(req, r)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found", "message":"record not found", "data": null}`))
		})
	})
})

type demoError struct {
	Message string
	Data    interface{}
}

func (e *demoError) Error() string {
	return fmt.Sprintf("demo error: %s", e.Message)
}
func (e *demoError) Respond() *bizerror.BizErrorDetail {
	return &bizerror.BizErrorDetail{
		Status: 444, Code: "common.demo",
		Message: e.Error(), Data: e.Data, Cause: nil,
	}
}
