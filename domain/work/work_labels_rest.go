package work

import (
	"flywheel/bizerror"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	PathWorkLabelRelations = "/v1/work-label-relations"
)

func RegisterWorkLabelRelationsRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathWorkLabelRelations, middleWares...)
	g.POST("", handleCreateWorkLabelRelation)
	g.DELETE("", handleDeleteWorkLabelRelation)
}

func handleCreateWorkLabelRelation(c *gin.Context) {
	req := WorkLabelRelationReq{}
	err := c.ShouldBindBodyWith(&req, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	record, err := CreateWorkLabelRelationFunc(req, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, record)
}

func handleDeleteWorkLabelRelation(c *gin.Context) {
	req := WorkLabelRelationReq{}
	err := c.ShouldBindQuery(&req)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = DeleteWorkLabelRelationFunc(req, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusNoContent)
}
