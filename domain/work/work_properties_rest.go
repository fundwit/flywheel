package work

import (
	"flywheel/bizerror"
	"flywheel/session"
	"net/http"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	PathWorkProperties = "/v1/work-properties"
)

func RegisterWorkPropertiesRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathWorkProperties, middleWares...)
	g.PATCH("", handleAssignWorkProperties)
	g.GET("", handleQueryWorkPropertyValues)
}

func handleAssignWorkProperties(c *gin.Context) {
	req := WorkPropertyAssign{}
	err := c.ShouldBindBodyWith(&req, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	record, err := AssignWorkPropertyValueFunc(req, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, record)
}

type workPropertyValuesQuery struct {
	WorkIds []types.ID `json:"workIds" form:"workId" binding:"gte=1"`
}

func handleQueryWorkPropertyValues(c *gin.Context) {
	req := workPropertyValuesQuery{}
	err := c.ShouldBindQuery(&req)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	result, err := QueryWorkPropertyValuesFunc(req.WorkIds, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, result)
}
