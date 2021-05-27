package security

import (
	"flywheel/bizerror"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterSessionHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/session", middleWares...)
	g.GET("", DetailSessionSecurityContext)
}

func DetailSessionSecurityContext(c *gin.Context) {
	sec := FindSecurityContext(c)

	now := time.Now()
	ttl := TokenExpiration - now.Sub(sec.SigningTime)
	if ttl > 0 {
		perms, projectRoles := LoadPermFunc(sec.Identity.ID)
		securityContext := Context{Token: sec.Token, Identity: sec.Identity, Perms: perms, ProjectRoles: projectRoles, SigningTime: now}
		TokenCache.Set(sec.Token, &securityContext, ttl)
		c.JSON(http.StatusOK, &securityContext)
	} else {
		panic(bizerror.ErrUnauthenticated)
	}
}
