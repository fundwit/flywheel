package sessions_test

import (
	"context"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/sessions"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
)

func TestDetailSessionSecurityContext(t *testing.T) {
	RegisterTestingT(t)

	var (
		router       *gin.Engine
		testDatabase *testinfra.TestDatabase
	)

	t.Run("should be able to fresh security context successfully", func(t *testing.T) {
		defer afterEachSessionRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionRestApiCase(t)

		Expect(testDatabase.DS.GormDB(context.Background()).Save(&account.User{ID: 2, Name: "ann", Secret: account.HashSha256("abc123")}).Error).To(BeNil())

		begin := time.Now()
		time.Sleep(1 * time.Millisecond)
		token := uuid.New().String()
		session.TokenCache.Set(token, &session.Session{Token: token, Identity: session.Identity{Name: "ann", Nickname: "Ann", ID: 1},
			Perms: []string{domain.ProjectRoleManager + "_1"}, ProjectRoles: []domain.ProjectRole{{
				Role: domain.ProjectRoleManager, ProjectName: "project1", ProjectIdentifier: "TES", ProjectID: types.ID(1),
			}}, SigningTime: time.Now()}, cache.DefaultExpiration)

		account.LoadPermFunc = func(uid types.ID) (authority.Permissions, authority.ProjectRoles) {
			return authority.Permissions{"manager_1"}, authority.ProjectRoles{{ProjectID: 1, ProjectName: "project1new", ProjectIdentifier: "TES", Role: "manager"}}
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
		req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: token})
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann", "nickname":"Ann"}, "token":"` + token +
			`", "perms":["manager_1"], "projectRoles":[{"projectId":"1", "projectName":"project1new", "projectIdentifier":"TES", "role":"manager"}]}`))

		// existed in token cache
		time.Sleep(1 * time.Millisecond)
		securityContextValue, found := session.TokenCache.Get(token)
		Expect(found).To(BeTrue())
		secCtx, ok := securityContextValue.(*session.Session)
		Expect(ok).To(BeTrue())
		Expect((*secCtx).SigningTime.After(begin) && (*secCtx).SigningTime.Before(time.Now()))
		Expect(*secCtx).To(Equal(session.Session{
			Token:        token,
			Identity:     session.Identity{ID: 1, Name: "ann", Nickname: "Ann"},
			Perms:        []string{"manager_1"},
			ProjectRoles: []domain.ProjectRole{{ProjectID: 1, ProjectName: "project1new", ProjectIdentifier: "TES", Role: "manager"}},
			SigningTime:  (*secCtx).SigningTime}))
	})

	t.Run("should be return 401 when token is invalid", func(t *testing.T) {
		defer afterEachSessionRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionRestApiCase(t)

		req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusUnauthorized))
		Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
	})

	t.Run("should be return 401 when token is timeout", func(t *testing.T) {
		defer afterEachSessionRestApiCase(t, testDatabase)
		router, testDatabase = beforeEachSessionRestApiCase(t)

		Expect(testDatabase.DS.GormDB(context.Background()).Save(&account.User{ID: 2, Name: "ann", Secret: account.HashSha256("abc123")}).Error).To(BeNil())
		token := uuid.New().String()
		session.TokenCache.Set(token, &session.Session{Token: token, Identity: session.Identity{Name: "ann", ID: 1},
			Perms: []string{domain.ProjectRoleManager + "_1"}, ProjectRoles: []domain.ProjectRole{{
				Role: domain.ProjectRoleManager + "", ProjectName: "project1", ProjectIdentifier: "TES", ProjectID: types.ID(1),
			}}, SigningTime: time.Now().AddDate(0, 0, -1)}, cache.DefaultExpiration)

		req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
		req.AddCookie(&http.Cookie{Name: session.KeySecToken, Value: token})
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusUnauthorized))
		Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
	})
}

func beforeEachSessionRestApiCase(t *testing.T) (*gin.Engine, *testinfra.TestDatabase) {
	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	securityMiddle := session.SimpleAuthFilter()
	sessions.RegisterSessionHandler(router, securityMiddle)
	session.TokenCache.Flush()
	testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
	persistence.ActiveDataSourceManager = testDatabase.DS

	Expect(testDatabase.DS.GormDB(context.Background()).AutoMigrate(&account.User{}, &domain.ProjectMember{},
		&account.Role{}, &account.Permission{}, &account.UserRoleBinding{}, &account.RolePermissionBinding{}).Error).To(BeNil())

	return router, testDatabase
}

func afterEachSessionRestApiCase(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}
