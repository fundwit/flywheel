package servehttp

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/misc"
	"flywheel/session"
	"net/http"
	"strconv"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

type TransitionQuery struct {
	FlowID    types.ID `uri:"flowId" json:"-" validate:"required,min=1"`
	FromState string   `form:"fromState"`
	ToState   string   `form:"toState"`
}

func RegisterWorkflowHandler(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/workflows", middleWares...)

	handler := &workflowHandler{
		validator: validator.New(),
	}

	g.POST("", handler.handleCreateWorkflow)
	g.GET("", handler.handleQueryWorkflows)
	g.GET(":flowId", handler.handleDetailWorkflows)
	g.PUT(":flowId", handler.handleUpdateWorkflowsBase)
	g.DELETE(":flowId", handler.handleDeleteWorkflow)

	g.POST(":flowId/states", handler.handleCreateStateMachineState)
	g.PUT(":flowId/states", handler.handleUpdateStateMachineState)
	g.PUT(":flowId/state-orders", handler.handleUpdateStateMachineStateOrders)

	g.GET(":flowId/transitions", handler.handleQueryTransitions)
	g.POST(":flowId/transitions", handler.handleCreateStateMachineTransitions)
	g.DELETE(":flowId/transitions", handler.handleDeleteStateMachineTransitions)

	g.GET(":flowId/properties", queryStateMachinePropertyRestAPI)
	g.POST(":flowId/properties", createStateMachinePropertyRestAPI)
}

type workflowHandler struct {
	validator *validator.Validate
}

func (h *workflowHandler) handleQueryWorkflows(c *gin.Context) {
	query := domain.WorkflowQuery{}
	_ = c.MustBindWith(&query, binding.Query)

	flows, err := flow.QueryWorkflowsFunc(&query, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, flows)
}

func (h *workflowHandler) handleCreateWorkflow(c *gin.Context) {
	creation := flow.WorkflowCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(creation); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	workflow, err := flow.CreateWorkflowFunc(&creation, session.ExtractSessionFromGinContext(c))
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
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	workflowDetail, err := flow.DetailWorkflowFunc(id, session.ExtractSessionFromGinContext(c))
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
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	updating := flow.WorkflowBaseUpdation{}
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(updating); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	workflow, err := flow.UpdateWorkflowBaseFunc(id, &updating, session.ExtractSessionFromGinContext(c))
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
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	err = flow.DeleteWorkflowFunc(id, session.ExtractSessionFromGinContext(c))
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
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(query); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	workflow, _ := flow.DetailWorkflowFunc(query.FlowID, session.ExtractSessionFromGinContext(c))
	if workflow == nil {
		c.JSON(http.StatusNotFound, &misc.ErrorBody{Code: "common.bad_param",
			Message: "the flow of id " + strconv.FormatUint(uint64(query.FlowID), 10) + " was not found"})
		return
	}

	availableTransitions := workflow.StateMachine.AvailableTransitions(query.FromState, query.ToState)
	c.JSON(http.StatusOK, availableTransitions)
}

func (h *workflowHandler) handleCreateStateMachineTransitions(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var transitions []state.Transition
	err = c.ShouldBindBodyWith(&transitions, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	for _, t := range transitions {
		if err = h.validator.Struct(t); err != nil {
			panic(&bizerror.ErrBadParam{Cause: err})
		}
	}

	err = flow.CreateWorkflowStateTransitionsFunc(id, transitions, session.ExtractSessionFromGinContext(c))
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
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var transitions []state.Transition
	err = c.ShouldBindBodyWith(&transitions, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	for _, t := range transitions {
		if err = h.validator.Struct(t); err != nil {
			panic(&bizerror.ErrBadParam{Cause: err})
		}
	}

	err = flow.DeleteWorkflowStateTransitionsFunc(id, transitions, session.ExtractSessionFromGinContext(c))
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
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var updating flow.WorkflowStateUpdating
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	if err = h.validator.Struct(updating); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = flow.UpdateWorkflowStateFunc(id, updating, session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *workflowHandler) handleUpdateStateMachineStateOrders(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var orderUpdating []flow.StateOrderRangeUpdating
	err = c.ShouldBindBodyWith(&orderUpdating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	for _, updating := range orderUpdating {
		if err = h.validator.Struct(updating); err != nil {
			panic(&bizerror.ErrBadParam{Cause: err})
		}
	}
	err = flow.UpdateStateRangeOrdersFunc(id, &orderUpdating, session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *workflowHandler) handleCreateStateMachineState(c *gin.Context) {
	id, err := types.ParseID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &misc.ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("flowId") + "'"})
		return
	}

	var stateCreating flow.StateCreating
	err = c.ShouldBindBodyWith(&stateCreating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = flow.CreateStateFunc(id, &stateCreating, session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
