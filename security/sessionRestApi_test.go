package security_test

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
)

var _ = Describe("SessionRestApi", func() {
	var (
		router       *gin.Engine
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		securityMiddle := security.SimpleAuthFilter()
		security.RegisterSessionHandler(router, securityMiddle)
		security.TokenCache.Flush()
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB().AutoMigrate(&security.User{}, &domain.ProjectMember{},
			&security.Role{}, &security.Permission{}, &security.UserRoleBinding{}, &security.RolePermissionBinding{}).Error).To(BeNil())
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("DetailSessionSecurityContext", func() {
		It("should be able to fresh security context successfully", func() {
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 2, Name: "ann", Secret: security.HashSha256("abc123")}).Error).To(BeNil())

			begin := time.Now()
			time.Sleep(1 * time.Millisecond)
			token := uuid.New().String()
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1},
				Perms: []string{"owner_1"}, ProjectRoles: []domain.ProjectRole{{
					Role: "owner", ProjectName: "project1", ProjectIdentifier: "TES", ProjectID: types.ID(1),
				}}, SigningTime: time.Now()}, cache.DefaultExpiration)

			security.LoadPermFunc = func(uid types.ID) ([]string, []domain.ProjectRole) {
				return []string{"manager_1"}, []domain.ProjectRole{{ProjectID: 1, ProjectName: "project1new", ProjectIdentifier: "TES", Role: "manager"}}
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann"}, "token":"` + token +
				`", "perms":["manager_1"], "projectRoles":[{"projectId":"1", "projectName":"project1new", "projectIdentifier":"TES", "role":"manager"}]}`))

			// existed in token cache
			time.Sleep(1 * time.Millisecond)
			securityContextValue, found := security.TokenCache.Get(token)
			Expect(found).To(BeTrue())
			secCtx, ok := securityContextValue.(*security.Context)
			Expect(ok).To(BeTrue())
			Expect((*secCtx).SigningTime.After(begin) && (*secCtx).SigningTime.Before(time.Now()))
			Expect(*secCtx).To(Equal(security.Context{
				Token:        token,
				Identity:     security.Identity{ID: 1, Name: "ann"},
				Perms:        []string{"manager_1"},
				ProjectRoles: []domain.ProjectRole{{ProjectID: 1, ProjectName: "project1new", ProjectIdentifier: "TES", Role: "manager"}},
				SigningTime:  (*secCtx).SigningTime}))
		})

		It("should be return 401 when token is invalid", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
		})

		It("should be return 401 when token is timeout", func() {
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 2, Name: "ann", Secret: security.HashSha256("abc123")}).Error).To(BeNil())
			token := uuid.New().String()
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1},
				Perms: []string{"owner_1"}, ProjectRoles: []domain.ProjectRole{{
					Role: "owner", ProjectName: "project1", ProjectIdentifier: "TES", ProjectID: types.ID(1),
				}}, SigningTime: time.Now().AddDate(0, 0, -1)}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
		})
	})
})
