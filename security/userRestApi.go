package security

import (
	"flywheel/bizerror"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"net/http"
)

var (
	UpdateBasicAuthSecretFunc = UpdateBasicAuthSecret
	QueryUsersFunc            = QueryUsers
	CreateUserFunc            = CreateUser
)

func RegisterSessionUsersHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	u := r.Group("/v1/session-users", middleWares...)

	u.PUT("basic-auths", HandleUpdateBaseAuth)

	users := r.Group("/v1/users", middleWares...)
	users.GET("", HandleQueryUsers)
	users.POST("", HandleCreateUser)
}

func HandleQueryUsers(c *gin.Context) {
	results, err := QueryUsersFunc(FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, results)
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

func HandleCreateUser(c *gin.Context) {
	payload := UserCreation{}
	err := c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	u, err := CreateUserFunc(&payload, FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, u)
}
