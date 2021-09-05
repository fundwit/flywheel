package label

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
	PathLabels = "/v1/labels"
)

func RegisterLabelsRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathLabels, middleWares...)
	g.POST("", handleCreateLabel)
	g.GET("", handleQueryLabels)
	g.DELETE(":id", handleDeleteLabel)
}

func handleCreateLabel(c *gin.Context) {
	creation := LabelCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	record, err := CreateLabelFunc(creation, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, record)
}

func handleQueryLabels(c *gin.Context) {
	query := LabelQuery{}
	err := c.MustBindWith(&query, binding.Query)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	record, err := QueryLabelsFunc(query, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, record)
}

func handleDeleteLabel(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	err = DeleteLabelFunc(parsedId, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusNoContent)
}
