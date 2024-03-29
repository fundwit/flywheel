package account

import (
	"flywheel/bizerror"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	UpdateBasicAuthSecretFunc = UpdateBasicAuthSecret
	QueryUsersFunc            = QueryUsers
	CreateUserFunc            = CreateUser
	UpdateUserFunc            = UpdateUser
)

func RegisterUsersHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	users := r.Group("/v1/users", middleWares...)
	users.GET("", HandleQueryUsers)
	users.POST("", HandleCreateUser)
	users.PUT(":id", HandleUpdateUser)

	me := r.Group("/me", middleWares...)
	me.GET("", UserInfoQueryHandler)

	sessionUsers := r.Group("/v1/session-users", middleWares...)
	sessionUsers.PUT("basic-auths", HandleUpdateBaseAuth)
}

func UserInfoQueryHandler(c *gin.Context) {
	s := session.ExtractSessionFromGinContext(c)
	if s == nil {
		panic(bizerror.ErrUnauthenticated)
	}
	c.JSON(http.StatusOK, &s)
}

func HandleQueryUsers(c *gin.Context) {
	results, err := QueryUsersFunc(session.ExtractSessionFromGinContext(c))
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

	err = UpdateBasicAuthSecretFunc(&payload, session.ExtractSessionFromGinContext(c))
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

	u, err := CreateUserFunc(&payload, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, u)
}

func HandleUpdateUser(c *gin.Context) {
	id, err := misc.BindingPathID(c)
	if err != nil {
		panic(err)
	}

	payload := UserUpdation{}
	err = c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = UpdateUserFunc(id, &payload, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
