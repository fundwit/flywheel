package security

import (
	"flywheel/bizerror"
	"flywheel/common"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"net/http"
)

var (
	UpdateBasicAuthSecretFunc = UpdateBasicAuthSecret
	QueryUsersFunc            = QueryUsers
	CreateUserFunc            = CreateUser
)

func RegisterUsersHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	users := r.Group("/v1/users", middleWares...)
	users.GET("", HandleQueryUsers)
	users.POST("", HandleCreateUser)

	me := r.Group("/me", middleWares...)
	me.GET("", UserInfoQueryHandler)

	sessionUsers := r.Group("/v1/session-users", middleWares...)
	sessionUsers.PUT("basic-auths", HandleUpdateBaseAuth)
}

func UserInfoQueryHandler(c *gin.Context) {
	secCtx := FindSecurityContext(c)
	if secCtx == nil {
		c.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "login failed"})
		return
	}
	c.JSON(http.StatusOK, &secCtx)
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
