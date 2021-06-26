package security

import (
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/persistence"
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
	token, _ := c.Cookie(KeySecToken) // ErrNoCookie
	if token != "" {
		TokenCache.Delete(token)
	}
	c.SetCookie(KeySecToken, "", -1, "/", "", false, false)
	c.AbortWithStatus(http.StatusNoContent)
}

func SimpleLoginHandler(c *gin.Context) {
	login := LoginRequest{}
	if err := c.ShouldBindBodyWith(&login, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}
	identity := Identity{}
	db := persistence.ActiveDataSourceManager.GormDB()
	if err := db.Model(&User{}).Where(&User{Name: login.Name, Secret: HashSha256(login.Password)}).Scan(&identity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			panic(bizerror.ErrUnauthenticated)
		}
		c.JSON(http.StatusInternalServerError, &common.ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	token := uuid.New().String()
	perms, projectRoles := LoadPermFunc(identity.ID)
	securityContext := Context{Token: token, Identity: identity, Perms: perms, ProjectRoles: projectRoles, SigningTime: time.Now()}
	TokenCache.Set(token, &securityContext, cache.DefaultExpiration)

	c.SetCookie(KeySecToken, token, int(TokenExpiration/time.Second), "/", "", false, false)
	c.JSON(http.StatusOK, &securityContext)
}
