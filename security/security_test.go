package security_test

import (
	"bytes"
	"errors"
	"flywheel/domain"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

var _ = Describe("Security", func() {
	Describe("FindSecurityContext", func() {
		It("should work as expected", func() {
			ctx := &gin.Context{}
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, "string value")
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, &security.Context{})
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, &security.Context{Token: "a token"})
			Expect(*security.FindSecurityContext(ctx)).To(Equal(security.Context{Token: "a token"}))
		})
	})

	Describe("SaveSecurityContext", func() {
		It("should work as expected", func() {
			ctx := &gin.Context{}
			security.SaveSecurityContext(ctx, nil)
			_, found := ctx.Get(security.KeySecCtx)
			Expect(found).To(BeFalse())

			security.SaveSecurityContext(ctx, &security.Context{})
			_, found = ctx.Get(security.KeySecCtx)
			Expect(found).To(BeFalse())

			security.SaveSecurityContext(ctx, &security.Context{Token: "a token"})
			val, found := ctx.Get(security.KeySecCtx)
			Expect(found).To(BeTrue())
			secCtx, ok := val.(*security.Context)
			Expect(ok).To(BeTrue())
			Expect(*secCtx).To(Equal(security.Context{Token: "a token"}))
		})
	})

	Describe("security http serve", func() {
		var (
			router       *gin.Engine
			testDatabase *testinfra.TestDatabase
		)
		BeforeEach(func() {
			router = gin.Default()
			security.RegisterWorkHandler(router)
			security.TokenCache.Flush()
			testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
			err := testDatabase.DS.GormDB().AutoMigrate(&security.User{}, &domain.GroupMember{}).Error
			if err != nil {
				log.Fatalf("database migration failed %v\n", err)
			}
			security.DB = testDatabase.DS.GormDB()
		})
		AfterEach(func() {
			testinfra.StopMysqlTestDatabase(testDatabase)
		})

		Describe("SimpleLoginHandler", func() {
			It("should be able to login successfully", func() {
				err := testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "ann", Secret: security.HashSha256("abc123")}).Error
				Expect(err).To(BeNil())

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
				Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann"}, "token":"` + token +
					`", "perms":[], "groupRoles":[]}`))
				Expect(resp.Cookies()[0].Name).To(Equal(security.KeySecToken))
				Expect(resp.Cookies()[0].Value).ToNot(BeEmpty())

				// existed in token cache
				securityContextValue, found := security.TokenCache.Get(resp.Cookies()[0].Value)
				Expect(found).To(BeTrue())
				secCtx, ok := securityContextValue.(*security.Context)
				Expect(ok).To(BeTrue())
				Expect(*secCtx).To(Equal(security.Context{Token: resp.Cookies()[0].Value, Identity: security.Identity{ID: 1, Name: "ann"},
					Perms: []string{}, GroupRoles: []domain.GroupRole{}}))
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
					Role: "owner", GroupName: "group1", GroupID: types.ID(1),
				}}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"identity":{"id":"1","name":"ann"}, "token":"` + token +
				`", "perms":["owner_1"], "groupRoles":[{"groupId":"1", "groupName":"group1", "role":"owner"}]}`))
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

	Describe("LoadPerms", func() {
		It("should return actual permissions when matched", func() {
			testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
			defer testinfra.StopMysqlTestDatabase(testDatabase)

			now := time.Now()
			Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.GroupMember{}, &domain.Group{}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 1, Name: "group1", Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 20, Name: "group20", Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())

			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 10, MemberId: 30, Role: "viewer", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 20, MemberId: 3, Role: "viewer", CreateTime: now}).Error).To(BeNil())

			security.DB = testDatabase.DS.GormDB()
			s, gr := security.LoadPerms(3)
			Expect(len(s)).To(Equal(2))
			Expect(s).To(Equal([]string{"owner_1", "viewer_20"}))
			Expect(gr).To(Equal([]domain.GroupRole{{GroupID: 1, GroupName: "group1", Role: "owner"}, {GroupID: 20, GroupName: "group20", Role: "viewer"}}))

			s, gr = security.LoadPerms(1)
			Expect(len(s)).To(Equal(0))
			Expect(len(gr)).To(Equal(0))
		})

		It("should failed when database access failed", func() {
			func() {
				defer func() {
					err := recover()
					Expect(err).To(Equal(errors.New("sql: database is closed")))
				}()
				security.LoadPerms(3)
			}()
		})

		It("should panic when group record is not found", func() {
			func() {
				testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
				defer testinfra.StopMysqlTestDatabase(testDatabase)
				Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.GroupMember{}, &domain.Group{}).Error).To(BeNil())
				security.DB = testDatabase.DS.GormDB()

				now := time.Now()
				Expect(testDatabase.DS.GormDB().Create(
					&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: now}).Error).To(BeNil())

				defer func() {
					err := recover()
					Expect(err).To(Equal(errors.New("group 1 is not exist")))
				}()

				security.LoadPerms(3)
			}()
		})

		It("should panic when group query failed", func() {
			func() {
				testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
				defer testinfra.StopMysqlTestDatabase(testDatabase)
				Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.GroupMember{}).Error).To(BeNil())
				security.DB = testDatabase.DS.GormDB()

				now := time.Now()
				Expect(testDatabase.DS.GormDB().Create(
					&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: now}).Error).To(BeNil())

				defer func() {
					ret := recover()
					err, ok := ret.(error)
					Expect(ok).To(BeTrue())
					Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".groups' doesn't exist"))
				}()

				security.LoadPerms(3)
			}()
		})
	})
})
