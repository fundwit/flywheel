package servehttp_test

import (
	"flywheel/servehttp"
	"flywheel/testinfra"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("WorkflowHandler", func() {
	var (
		router *gin.Engine
	)

	BeforeEach(func() {
		router = gin.Default()
		servehttp.RegisterWorkflowHandler(router)
	})

	Describe("handleQueryStates", func() {
		It("should be able to query states success", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name": "PENDING"},{"name": "DOING"},{"name": "DONE"}]`))
		})

		It("should return 404 when workflow is not exists", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"the flow of id 2 was not found", "data": null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"strconv.ParseUint: parsing \"abc\": invalid syntax", "data": null}`))
		})

		It("should return 400 when validate failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/0/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", 
				"message":"invalid flowId '0'", "data": null}`))
		})
	})

	Describe("handleQueryTransitions", func() {
		It("should be able to query transitions with query: fromState", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=PENDING", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"begin","from":{"name": "PENDING"},"to":{"name": "DOING"}},
				{"name":"close","from":{"name": "PENDING"},"to":{"name": "DONE"}}]`))
		})

		It("should be able to query transitions with query: fromState and toState", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=PENDING&toState=DONE", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"close","from":{"name": "PENDING"},"to":{"name": "DONE"}}]`))
		})

		It("should be able to query transitions with unknown state", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=UNKNOWN", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[]`))
		})

		It("should return 404 when workflow is not exists", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"the flow of id 2 was not found", "data": null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"strconv.ParseUint: parsing \"abc\": invalid syntax", "data": null}`))
		})

		It("should return 400 when validate failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/0/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", 
				"message":"Key: 'TransitionQuery.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag", "data": null}`))
		})
	})
})
