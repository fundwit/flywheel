package sessions_test

import (
	"bytes"
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/sessions"
	"flywheel/testinfra"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
)

func TestSimpleLoginHandler(t *testing.T) {
	RegisterTestingT(t)

	var (
		router       *gin.Engine
		testDatabase *testinfra.TestDatabase
	)

	t.Run("should be able to login successfully", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		Expect(testDatabase.DS.GormDB().Save(&account.User{ID: 2, Name: "ann", Nickname: "Ann", Secret: account.HashSha256("abc123")}).Error).To(BeNil())

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
		for k := range session.TokenCache.Items() {
			token = k
			break
		}
		Expect(body).To(MatchJSON(`{"identity":{"id":"2","name":"ann", "nickname":"Ann"}, "token":"` + token +
			`", "perms":[], "projectRoles":[]}`))
		Expect(resp.Cookies()[0].Name).To(Equal(session.KeySecToken))
		Expect(resp.Cookies()[0].Value).ToNot(BeEmpty())

		// existed in token cache
		time.Sleep(1 * time.Millisecond)
		securityContextValue, found := session.TokenCache.Get(resp.Cookies()[0].Value)
		Expect(found).To(BeTrue())
		secCtx, ok := securityContextValue.(*session.Context)
		Expect(ok).To(BeTrue())
		Expect((*secCtx).SigningTime.After(begin) && (*secCtx).SigningTime.Before(time.Now()))
		Expect(*secCtx).To(Equal(session.Context{Token: resp.Cookies()[0].Value, Identity: session.Identity{ID: 2, Name: "ann", Nickname: "Ann"},
			Perms: []string{}, ProjectRoles: []domain.ProjectRole{}, SigningTime: (*secCtx).SigningTime}))
	})

	t.Run("should return 401 when user not exist", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"abc123"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusUnauthorized))
		Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
	})

	t.Run("should return 401 when user password is not correct", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		err := testDatabase.DS.GormDB().Save(&account.User{ID: 1, Name: "ann", Secret: account.HashSha256("abc123")}).Error
		Expect(err).To(BeNil())

		req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusUnauthorized))
		Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
	})

	t.Run("should return 400 when bind failed", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`bad json`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 500 when query failed", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		err := testDatabase.DS.GormDB().DropTable(&account.User{}).Error
		Expect(err).To(BeNil())

		req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"Error 1146: Table '` +
			testDatabase.TestDatabaseName + `.users' doesn't exist","data":null}`))
	})
}

func TestSimpleLogoutHandler(t *testing.T) {
	RegisterTestingT(t)

	var (
		router       *gin.Engine
		testDatabase *testinfra.TestDatabase
	)

	t.Run("should return 204 when token is cleared", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		Expect(session.TokenCache.Add("test_token", &session.Context{}, cache.DefaultExpiration)).To(BeNil())
		_, found := session.TokenCache.Get("test_token")
		Expect(found).To(BeTrue())

		req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
		req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: "test_token"})
		status, body, resp := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
		Expect(len(resp.Cookies())).To(Equal(1))
		cookie := resp.Cookies()[0]
		Expect(cookie.Name).To(Equal(session.KeySecToken))
		Expect(cookie.Value).To(BeEmpty())
		Expect(cookie.MaxAge).To(Equal(-1))

		_, found = session.TokenCache.Get("test_token")
		Expect(found).To(BeFalse())
	})

	t.Run("should return 204 when token is not found too", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		Expect(session.TokenCache.Add("test_token", &session.Context{}, cache.DefaultExpiration)).To(BeNil())
		_, found := session.TokenCache.Get("test_token")
		Expect(found).To(BeTrue())

		req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
		req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: "test_token123"})
		status, body, resp := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
		Expect(len(resp.Cookies())).To(Equal(1))
		cookie := resp.Cookies()[0]
		Expect(cookie.Name).To(Equal(session.KeySecToken))
		Expect(cookie.Value).To(BeEmpty())
		Expect(cookie.MaxAge).To(Equal(-1))

		_, found = session.TokenCache.Get("test_token")
		Expect(found).To(BeTrue())
	})

	t.Run("should return 204 when request without token", func(t *testing.T) {
		defer afterEachSessionsRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionsRestApiCase(t)

		Expect(session.TokenCache.Add("test_token", &session.Context{}, cache.DefaultExpiration)).To(BeNil())
		_, found := session.TokenCache.Get("test_token")
		Expect(found).To(BeTrue())

		req := httptest.NewRequest(http.MethodDelete, "/v1/sessions", nil)
		status, body, resp := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
		Expect(len(resp.Cookies())).To(Equal(1))
		cookie := resp.Cookies()[0]
		Expect(cookie.Name).To(Equal(session.KeySecToken))
		Expect(cookie.Value).To(BeEmpty())
		Expect(cookie.MaxAge).To(Equal(-1))

		_, found = session.TokenCache.Get("test_token")
		Expect(found).To(BeTrue())
	})
}

func beforeEachSessionsRestApiCase(t *testing.T) (*gin.Engine, *testinfra.TestDatabase) {
	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	sessions.RegisterSessionsHandler(router)
	session.TokenCache.Flush()
	testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
	persistence.ActiveDataSourceManager = testDatabase.DS
	Expect(testDatabase.DS.GormDB().AutoMigrate(&account.User{}, &domain.ProjectMember{},
		&account.Role{}, &account.Permission{}, &account.UserRoleBinding{}, &account.RolePermissionBinding{}).Error).To(BeNil())
	account.LoadPermFuncReset()

	return router, testDatabase
}

func afterEachSessionsRestApiCase(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}
