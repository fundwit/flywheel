package sessions

import (
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/misc"
	"flywheel/persistence"
	"flywheel/session"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/patrickmn/go-cache"
)

func RegisterSessionsHandler(r *gin.Engine) {
	g := r.Group("/v1/sessions")
	g.POST("", SimpleLoginHandler)
	g.DELETE("", SimpleLogoutHandler)
}

func SimpleLogoutHandler(c *gin.Context) {
	token, _ := c.Cookie(session.KeySecToken) // ErrNoCookie
	if token != "" {
		session.TokenCache.Delete(token)
	}
	c.SetCookie(session.KeySecToken, "", -1, "/", "", false, false)
	c.AbortWithStatus(http.StatusNoContent)
}

func SimpleLoginHandler(c *gin.Context) {
	login := session.LoginRequest{}
	if err := c.ShouldBindBodyWith(&login, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}
	identity := session.Identity{}
	db := persistence.ActiveDataSourceManager.GormDB(c.Request.Context())
	if err := db.Model(&account.User{}).Where(&account.User{Name: login.Name, Secret: account.HashSha256(login.Password)}).Scan(&identity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			panic(bizerror.ErrUnauthenticated)
		}
		c.JSON(http.StatusInternalServerError, &misc.ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	token := uuid.New().String()
	perms, projectRoles := account.LoadPermFunc(identity.ID)
	securityContext := session.Session{Token: token, Identity: identity, Perms: perms, ProjectRoles: projectRoles, SigningTime: time.Now()}
	session.TokenCache.Set(token, &securityContext, cache.DefaultExpiration)

	c.SetCookie(session.KeySecToken, token, int(session.TokenExpiration/time.Second), "/", "", false, false)
	c.JSON(http.StatusOK, &securityContext)
}
