package security_test

import (
	"bytes"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
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
			router.POST("/login", security.SimpleLoginHandler)
			testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
			err := testDatabase.DS.GormDB().AutoMigrate(&security.User{}).Error
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

				req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"name": "ann", "password":"abc123"}`)))
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				resp := w.Result()
				defer resp.Body.Close()
				bodyBytes, _ := ioutil.ReadAll(resp.Body)
				body := string(bodyBytes)
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(body).To(MatchJSON(`{"id":"1", "name":"ann"}`))
				Expect(resp.Cookies()[0].Name).To(Equal(security.KeySecToken))
				Expect(resp.Cookies()[0].Value).ToNot(BeEmpty())
			})

			It("should return 401 when user not exist", func() {
				req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"name": "ann", "password":"abc123"}`)))
				status, body, _ := testinfra.ExecuteRequest(req, router)
				Expect(status).To(Equal(http.StatusUnauthorized))
				Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
			})

			It("should return 401 when user password is not correct", func() {
				err := testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "ann", Secret: security.HashSha256("abc123")}).Error
				Expect(err).To(BeNil())

				req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
				status, body, _ := testinfra.ExecuteRequest(req, router)
				Expect(status).To(Equal(http.StatusUnauthorized))
				Expect(body).To(MatchJSON(`{"code":"common.unauthenticated","message":"unauthenticated","data":null}`))
			})

			It("should return 400 when bind failed", func() {
				req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`bad json`)))
				status, body, _ := testinfra.ExecuteRequest(req, router)
				Expect(status).To(Equal(http.StatusBadRequest))
				Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
			})

			It("should return 500 when query failed", func() {
				err := testDatabase.DS.GormDB().DropTable(&security.User{}).Error
				Expect(err).To(BeNil())

				req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"name": "ann", "password":"bad pass"}`)))
				status, body, _ := testinfra.ExecuteRequest(req, router)
				Expect(status).To(Equal(http.StatusInternalServerError))
				Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"Error 1146: Table '` +
					testDatabase.TestDatabaseName + `.users' doesn't exist","data":null}`))
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
			security.TokenCache.Set(token, &security.Context{Token: token, Identity: security.Identity{Name: "ann", ID: 1}}, cache.DefaultExpiration)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.AddCookie(&http.Cookie{Name: security.KeySecToken, Value: token})
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"1","name":"ann"}`))
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
})
