package security

import (
	"flywheel/bizerror"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"net/http"
)

var (
	UpdateBasicAuthSecretFunc = UpdateBasicAuthSecret
)

func RegisterSessionUsersHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	u := r.Group("/v1/session-users", middleWares...)

	u.PUT("basic-auths", HandleUpdateBaseAuth)
}

func HandleUpdateBaseAuth(c *gin.Context) {
	payload := BasicAuthUpdating{}
	err := c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = UpdateBasicAuthSecretFunc(&payload, FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
