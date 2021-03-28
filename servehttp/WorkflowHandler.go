package servehttp

import (
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/security"
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

func RegisterWorkflowHandler(r *gin.Engine, flowManager flow.WorkflowManagerTraits, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/workflows", middleWares...)

	handler := &workflowHandler{
		validator:       validator.New(),
		workflowManager: flowManager,
	}

	g.POST("", handler.handleCreateWorkflow)
	g.GET("", handler.handleQueryWorkflows)
	g.GET(":flowId", handler.handleDetailWorkflows)
	g.PUT(":flowId", handler.handleUpdateWorkflowsBase)
	g.DELETE(":flowId", handler.handleDeleteWorkflow)

	g.PUT(":flowId/states", handler.handleUpdateStateMachineState)

	g.GET(":flowId/transitions", handler.handleQueryTransitions)
	g.POST(":flowId/transitions", handler.handleCreateStateMachineTransitions)
	g.DELETE(":flowId/transitions", handler.handleDeleteStateMachineTransitions)
}

type workflowHandler struct {
	validator       *validator.Validate
	workflowManager flow.WorkflowManagerTraits
}

func (h *workflowHandler) handleQueryWorkflows(c *gin.Context) {
	query := domain.WorkflowQuery{}
	_ = c.MustBindWith(&query, binding.Query)

	flows, err := h.workflowManager.QueryWorkflows(&query, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, flows)
}

func (h *workflowHandler) handleCreateWorkflow(c *gin.Context) {
	creation := flow.WorkflowCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(creation); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	workflow, err := h.workflowManager.CreateWorkflow(&creation, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, workflow)
}

func (h *workflowHandler) handleDetailWorkflows(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	workflowDetail, err := h.workflowManager.DetailWorkflow(id, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, workflowDetail)
}

func (h *workflowHandler) handleUpdateWorkflowsBase(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	updating := flow.WorkflowBaseUpdation{}
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(updating); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	workflow, err := h.workflowManager.UpdateWorkflowBase(id, &updating, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, workflow)
}

func (h *workflowHandler) handleDeleteWorkflow(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	err = h.workflowManager.DeleteWorkflow(id, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
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

	workflow, err := h.workflowManager.DetailWorkflow(query.FlowID, security.FindSecurityContext(c))
	if workflow == nil {
		c.JSON(http.StatusNotFound, &common.ErrorBody{Code: "common.bad_param",
			Message: "the flow of id " + strconv.FormatUint(uint64(query.FlowID), 10) + " was not found"})
		return
	}

	availableTransitions := workflow.StateMachine.AvailableTransitions(query.FromState, query.ToState)
	c.JSON(http.StatusOK, availableTransitions)
}

func (h *workflowHandler) handleCreateStateMachineTransitions(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var transitions []state.Transition
	err = c.ShouldBindBodyWith(&transitions, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	for _, t := range transitions {
		if err = h.validator.Struct(t); err != nil {
			panic(&common.ErrBadParam{Cause: err})
		}
	}

	err = h.workflowManager.CreateWorkflowStateTransitions(id, transitions, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *workflowHandler) handleDeleteStateMachineTransitions(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var transitions []state.Transition
	err = c.ShouldBindBodyWith(&transitions, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	for _, t := range transitions {
		if err = h.validator.Struct(t); err != nil {
			panic(&common.ErrBadParam{Cause: err})
		}
	}

	err = h.workflowManager.DeleteWorkflowStateTransitions(id, transitions, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *workflowHandler) handleUpdateStateMachineState(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var updating flow.WorkflowStateUpdating
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(updating); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	err = h.workflowManager.UpdateWorkflowState(id, updating, security.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
