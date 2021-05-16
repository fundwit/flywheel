package security_test

import (
	"bytes"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
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
		security.RegisterUsersHandler(router)
	})

	Describe("UserInfoQueryHandler", func() {
		var (
			router *gin.Engine
		)
		BeforeEach(func() {
			router = gin.Default()
			router.GET("/me", security.SimpleAuthFilter(), security.UserInfoQueryHandler)
		})
		It("should success when token is valid", func() {
			token := uuid.New().String()
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1},
				Perms: []string{"owner_1"}, GroupRoles: []domain.GroupRole{{
					Role: "owner", GroupName: "group1", GroupIdentifier: "TES", GroupID: types.ID(1),
				}}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann"}, "token":"` + token +
				`", "perms":["owner_1"], "groupRoles":[{"groupId":"1", "groupName":"group1", "groupIdentifier":"TES", "role":"owner"}]}`))
		})

		It("should failed when token is missing", func() {
			token := uuid.New().String()
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated", "data": null}`))
		})

		It("should failed when token not match", func() {
			token := uuid.New().String()
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: "bad token"})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated", "data": null}`))
		})
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
