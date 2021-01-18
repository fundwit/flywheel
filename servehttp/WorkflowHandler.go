package servehttp

import (
	"errors"
	"flywheel/common"
	"flywheel/domain"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
	"strconv"
)

type TransitionQuery struct {
	FlowID    types.ID `uri:"flowId" json:"-" validate:"required,min=1"`
	FromState string   `form:"fromState"`
	ToState   string   `form:"toState"`
}

func RegisterWorkflowHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	// group: "", version: v1, resource: transitions
	g := r.Group("/v1/workflows", middleWares...)

	handler := &workflowHandler{validator: validator.New()}

	g.GET(":flowId/transitions", handler.handleQueryTransitions)
	g.GET(":flowId/states", handler.handleQueryStates)
}

type workflowHandler struct {
	validator *validator.Validate
}

func (h *workflowHandler) handleQueryStates(c *gin.Context) {
	query := TransitionQuery{}
	err := c.ShouldBindUri(&query)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	if query.FlowID <= 0 {
		panic(&common.ErrBadParam{Cause: errors.New("invalid flowId '" + c.Params.ByName("flowId") + "'")})
	}
	workflow := domain.FindWorkflow(query.FlowID)
	if workflow == nil {
		c.JSON(http.StatusNotFound, &common.ErrorBody{Code: "common.bad_param",
			Message: "the flow of id " + strconv.FormatUint(uint64(query.FlowID), 10) + " was not found"})
		return
	}

	states := workflow.StateMachine.States
	c.JSON(http.StatusOK, states)
}

func (h *workflowHandler) handleQueryTransitions(c *gin.Context) {
	query := TransitionQuery{}
	_ = c.MustBindWith(&query, binding.Form)
	err := c.ShouldBindUri(&query)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(query); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	workflow := domain.FindWorkflow(query.FlowID)
	if workflow == nil {
		c.JSON(http.StatusNotFound, &common.ErrorBody{Code: "common.bad_param",
			Message: "the flow of id " + strconv.FormatUint(uint64(query.FlowID), 10) + " was not found"})
		return
	}

	availableTransitions := workflow.StateMachine.AvailableTransitions(query.FromState, query.ToState)
	c.JSON(http.StatusOK, availableTransitions)
}
