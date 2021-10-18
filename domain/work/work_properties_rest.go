package work

import (
	"flywheel/bizerror"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	PathWorkProperties = "/v1/work-properties"
)

func RegisterWorkPropertiesRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathWorkProperties, middleWares...)
	g.PATCH("", handleAssignWorkProperties)
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
