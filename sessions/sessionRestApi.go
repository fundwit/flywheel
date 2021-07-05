package sessions

import (
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/session"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterSessionHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/session", middleWares...)
	g.GET("", DetailSessionSecurityContext)
}

func DetailSessionSecurityContext(c *gin.Context) {
	sec := session.FindSecurityContext(c)

	now := time.Now()
	ttl := session.TokenExpiration - now.Sub(sec.SigningTime)
	if ttl > 0 {
		perms, projectRoles := account.LoadPermFunc(sec.Identity.ID)
		securityContext := session.Context{Token: sec.Token, Identity: sec.Identity, Perms: perms, ProjectRoles: projectRoles, SigningTime: now}
		session.TokenCache.Set(sec.Token, &securityContext, ttl)
		c.JSON(http.StatusOK, &securityContext)
	} else {
		panic(bizerror.ErrUnauthenticated)
	}
}
