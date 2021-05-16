package security_test

import (
	"bytes"
	"flywheel/bizerror"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("UserRestApi", func() {
	var (
		router *gin.Engine
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		security.RegisterSessionUsersHandler(router)
	})

	Describe("HandleUpdateBaseAuth", func() {
		It("should return 200 when update successful", func() {
			var payload *security.BasicAuthUpdating
			security.UpdateBasicAuthSecretFunc = func(u *security.BasicAuthUpdating, sec *security.Context) error {
				payload = u
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/v1/session-users/basic-auths", bytes.NewReader([]byte(
				`{"originalSecret":"123456","newSecret":"654321"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeZero())

			Expect(*payload).To(Equal(security.BasicAuthUpdating{OriginalSecret: "123456", NewSecret: "654321"}))
		})

		It("should return 400 when validation failed", func() {
			var payload *security.BasicAuthUpdating
			security.UpdateBasicAuthSecretFunc = func(u *security.BasicAuthUpdating, sec *security.Context) error {
				payload = u
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/v1/session-users/basic-auths", bytes.NewReader([]byte(
				`{"originalSecret":"123","newSecret":"321"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
				"code":"common.bad_param",
				"message":"Key: 'BasicAuthUpdating.NewSecret' Error:Field validation for 'NewSecret' failed on the 'gte' tag",
				"data":null}`))
			Expect(payload).To(BeNil())
		})
	})

	Describe("HandleQueryUsers", func() {
		It("should return 200 when query successful", func() {
			security.QueryUsersFunc = func(sec *security.Context) (*[]security.UserInfo, error) {
				return &[]security.UserInfo{{ID: 123, Name: "test"}}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"id": "123", "name": "test"}]`))
		})
	})

	Describe("HandleCreateUser", func() {
		It("should return 200 when create successful", func() {
			var payload *security.UserCreation
			security.CreateUserFunc = func(c *security.UserCreation, sec *security.Context) (*security.UserInfo, error) {
				payload = c
				return &security.UserInfo{ID: 123, Name: "test"}, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader([]byte(
				`{"name":"test","secret":"123456"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test"}`))
			Expect(*payload).To(Equal(security.UserCreation{Name: "test", Secret: "123456"}))
		})
	})
})
