package servehttp

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func createStateMachinePropertyRestAPI(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var prop domain.PropertyDefinition
	err = c.ShouldBindBodyWith(&prop, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	p, err := flow.CreatePropertyDefinitionFunc(id, prop, session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, p)
}

func queryStateMachinePropertyRestAPI(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	props, err := flow.QueryPropertyDefinitionsFunc(id, session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, props)
}
