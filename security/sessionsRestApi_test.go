package security_test

import (
	"bytes"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/testinfra"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
)

var _ = Describe("SessionsRestApi", func() {
	var (
		router       *gin.Engine
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		security.RegisterSessionsHandler(router)
		security.TokenCache.Flush()
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB().AutoMigrate(&security.User{}, &domain.ProjectMember{},
			&security.Role{}, &security.Permission{}, &security.UserRoleBinding{}, &security.RolePermissionBinding{}).Error).To(BeNil())
		security.LoadPermFuncReset()
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("SimpleLoginHandler", func() {
		It("should be able to login successfully", func() {
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 2, Name: "ann", Nickname: "Ann", Secret: security.HashSha256("abc123")}).Error).To(BeNil())

			begin := time.Now()
			time.Sleep(1 * time.Millisecond)
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"abc123"}`)))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			defer func() {
				_ = resp.Body.Close()
			}()
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			body := string(bodyBytes)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			token := ""
			for k := range security.TokenCache.Items() {
				token = k
				break
			}
			Expect(body).To(MatchJSON(`{"identity":{"id":"2","name":"ann", "nickname":"Ann"}, "token":"` + token +
				`", "perms":[], "projectRoles":[]}`))
			Expect(resp.Cookies()[0].Name).To(Equal(security.KeySecToken))
			Expect(resp.Cookies()[0].Value).ToNot(BeEmpty())

			// existed in token cache
			time.Sleep(1 * time.Millisecond)
			securityContextValue, found := security.TokenCache.Get(resp.Cookies()[0].Value)
			Expect(found).To(BeTrue())
			secCtx, ok := securityContextValue.(*security.Context)
			Expect(ok).To(BeTrue())
			Expect((*secCtx).SigningTime.After(begin) && (*secCtx).SigningTime.Before(time.Now()))
			Expect(*secCtx).To(Equal(security.Context{Token: resp.Cookies()[0].Value, Identity: security.Identity{ID: 2, Name: "ann", Nickname: "Ann"},
				Perms: []string{}, ProjectRoles: []domain.ProjectRole{}, SigningTime: (*secCtx).SigningTime}))
		})

		It("should return 401 when user not exist", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"abc123"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
		})

		It("should return 401 when user password is not correct", func() {
			err := testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "ann", Secret: security.HashSha256("abc123")}).Error
			Expect(err).To(BeNil())

			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 500 when query failed", func() {
			err := testDatabase.DS.GormDB().DropTable(&security.User{}).Error
			Expect(err).To(BeNil())

			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"Error 1146: Table '` +
				testDatabase.TestDatabaseName + `.users' doesn't exist","data":null}`))
		})
	})

	Describe("SimpleLogoutHandler", func() {
		It("should return 204 when token is cleared", func() {
			Expect(security.TokenCache.Add("test_token", &security.Context{}, cache.DefaultExpiration)).To(BeNil())
			_, found := security.TokenCache.Get("test_token")
			Expect(found).To(BeTrue())

			req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: "test_token"})
			status, body, resp := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
			Expect(len(resp.Cookies())).To(Equal(1))
			cookie := resp.Cookies()[0]
			Expect(cookie.Name).To(Equal(security.KeySecToken))
			Expect(cookie.Value).To(BeEmpty())
			Expect(cookie.MaxAge).To(Equal(-1))

			_, found = security.TokenCache.Get("test_token")
			Expect(found).To(BeFalse())
		})

		It("should return 204 when token is not found too", func() {
			Expect(security.TokenCache.Add("test_token", &security.Context{}, cache.DefaultExpiration)).To(BeNil())
			_, found := security.TokenCache.Get("test_token")
			Expect(found).To(BeTrue())

			req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: "test_token123"})
			status, body, resp := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
			Expect(len(resp.Cookies())).To(Equal(1))
			cookie := resp.Cookies()[0]
			Expect(cookie.Name).To(Equal(security.KeySecToken))
			Expect(cookie.Value).To(BeEmpty())
			Expect(cookie.MaxAge).To(Equal(-1))

			_, found = security.TokenCache.Get("test_token")
			Expect(found).To(BeTrue())
		})

		It("should return 204 when request without token", func() {
			Expect(security.TokenCache.Add("test_token", &security.Context{}, cache.DefaultExpiration)).To(BeNil())
			_, found := security.TokenCache.Get("test_token")
			Expect(found).To(BeTrue())

			req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
			status, body, resp := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
			Expect(len(resp.Cookies())).To(Equal(1))
			cookie := resp.Cookies()[0]
			Expect(cookie.Name).To(Equal(security.KeySecToken))
			Expect(cookie.Value).To(BeEmpty())
			Expect(cookie.MaxAge).To(Equal(-1))

			_, found = security.TokenCache.Get("test_token")
			Expect(found).To(BeTrue())
		})
	})
})
