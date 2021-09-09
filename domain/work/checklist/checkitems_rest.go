package checklist

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/session"
	"net/http"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	PathCheckItems = "/v1/checkitems"
)

func RegisterCheckItemsRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathCheckItems, middleWares...)
	g.POST("", handleCreateCheckItem)
	g.DELETE(":id", handleDeleteCheckItem)
}

func handleCreateCheckItem(c *gin.Context) {
	req := CheckItemCreation{}
	err := c.ShouldBindBodyWith(&req, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	record, err := CreateCheckItemFunc(req, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, record)
}

func handleDeleteCheckItem(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	err = DeleteCheckItemFunc(parsedId, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusNoContent)
}
