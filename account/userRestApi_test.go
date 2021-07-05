package account_test

import (
	"bytes"
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
)

var _ = Describe("UserRestApi", func() {
	var (
		router *gin.Engine
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		account.RegisterUsersHandler(router)
	})

	Describe("UserInfoQueryHandler", func() {
		var (
			router *gin.Engine
		)
		BeforeEach(func() {
			router = gin.Default()
			router.Use(bizerror.ErrorHandling())
			router.GET("/me", session.SimpleAuthFilter(), account.UserInfoQueryHandler)
		})
		It("should success when token is valid", func() {
			token := uuid.New().String()
			session.TokenCache.Set(token, &session.Context{Token: token, Identity: session.Identity{Name: "ann", ID: 1},
				Perms: []string{domain.ProjectRoleManager + "_1"}, ProjectRoles: []domain.ProjectRole{{
					Role: domain.ProjectRoleManager, ProjectName: "project1", ProjectIdentifier: "TES", ProjectID: types.ID(1),
				}}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann", "nickname":""}, "token":"` + token +
				`", "perms":["` + domain.ProjectRoleManager + `_1"], "projectRoles":[{"projectId":"1", "projectName":"project1", "projectIdentifier":"TES", "role":"` + domain.ProjectRoleManager + `"}]}`))
		})

		It("should failed when token is missing", func() {
			token := uuid.New().String()
			session.TokenCache.Set(token, &session.Context{Token: token, Identity: session.Identity{Name: "ann", ID: 1}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated", "data": null}`))
		})

		It("should failed when token not match", func() {
			token := uuid.New().String()
			session.TokenCache.Set(token, &session.Context{Token: token, Identity: session.Identity{Name: "ann", ID: 1}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: "bad token"})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated", "data": null}`))
		})
	})

	Describe("HandleUpdateBaseAuth", func() {
		It("should return 200 when update successful", func() {
			var payload *account.BasicAuthUpdating
			account.UpdateBasicAuthSecretFunc = func(u *account.BasicAuthUpdating, sec *session.Context) error {
				payload = u
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/v1/session-users/basic-auths", bytes.NewReader([]byte(
				`{"originalSecret":"123456","newSecret":"654321"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeZero())

			Expect(*payload).To(Equal(account.BasicAuthUpdating{OriginalSecret: "123456", NewSecret: "654321"}))
		})

		It("should return 400 when validation failed", func() {
			var payload *account.BasicAuthUpdating
			account.UpdateBasicAuthSecretFunc = func(u *account.BasicAuthUpdating, sec *session.Context) error {
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
			account.QueryUsersFunc = func(sec *session.Context) (*[]account.UserInfo, error) {
				return &[]account.UserInfo{{ID: 123, Name: "test"}}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"id": "123", "name": "test", "nickname": ""}]`))
		})
	})

	Describe("HandleCreateUser", func() {
		It("should return 200 when create successful", func() {
			var payload *account.UserCreation
			account.CreateUserFunc = func(c *account.UserCreation, sec *session.Context) (*account.UserInfo, error) {
				payload = c
				return &account.UserInfo{ID: 123, Name: "test", Nickname: "Test"}, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader([]byte(
				`{"name":"test","secret":"123456", "nickname": "Test"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test", "nickname": "Test"}`))
			Expect(*payload).To(Equal(account.UserCreation{Name: "test", Secret: "123456", Nickname: "Test"}))
		})

		It("should return 200 when create successful with requried params", func() {
			var payload *account.UserCreation
			account.CreateUserFunc = func(c *account.UserCreation, sec *session.Context) (*account.UserInfo, error) {
				payload = c
				return &account.UserInfo{ID: 123, Name: "test"}, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader([]byte(
				`{"name":"test","secret":"123456"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test", "nickname": ""}`))
			Expect(*payload).To(Equal(account.UserCreation{Name: "test", Secret: "123456", Nickname: ""}))
		})
	})

	Describe("HandleUpdateUser", func() {
		It("should return 200 when update user successful", func() {
			var pathId types.ID
			var payload *account.UserUpdation
			account.UpdateUserFunc = func(id types.ID, c *account.UserUpdation, sec *session.Context) error {
				pathId = id
				payload = c
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/v1/users/123", bytes.NewReader([]byte(`{"nickname":"New name"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeZero())

			Expect(pathId).To(Equal(types.ID(123)))
			Expect(*payload).To(Equal(account.UserUpdation{Nickname: "New name"}))
		})

		It("should return 400 when validation failed", func() {
			var pathId types.ID
			var payload *account.UserUpdation
			account.UpdateUserFunc = func(id types.ID, c *account.UserUpdation, sec *session.Context) error {
				pathId = id
				payload = c
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/v1/users/123", bytes.NewReader([]byte(`{"nickname":null}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
				"code":"common.bad_param",
				"message":"Key: 'UserUpdation.Nickname' Error:Field validation for 'Nickname' failed on the 'required' tag",
				"data":null}`))
			Expect(pathId).To(Equal(types.ID(0)))
			Expect(payload).To(BeNil())

			req = httptest.NewRequest(http.MethodPut, "/v1/users/123",
				bytes.NewReader([]byte(`{"nickname":"ABCDEFGHIJKLMNOPQRSTUVWXYZ-ABCDEFGHIJKLMNOPQRSTUVWXYZ"}`)))
			status, body, _ = testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
				"code":"common.bad_param",
				"message":"Key: 'UserUpdation.Nickname' Error:Field validation for 'Nickname' failed on the 'lte' tag",
				"data":null}`))
			Expect(pathId).To(Equal(types.ID(0)))
			Expect(payload).To(BeNil())
		})
	})
})
