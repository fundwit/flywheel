package servehttp

import (
	"flywheel/domain/flow"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func RegisterWorkStateTransitionHer(r *gin.Engine, m flow.WorkflowManagerTraits, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/transitions", middleWares...)

	handler := &workStateTransitionHandler{manager: m, validator: validator.New()}
	g.POST("", handler.handleCreate)
}

type workStateTransitionHandler struct {
	manager   flow.WorkflowManagerTraits
	validator *validator.Validate
}

func (h *workStateTransitionHandler) handleCreate(c *gin.Context) {
	creation := flow.WorkStateTransitionBrief{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}

	if err = h.validator.Struct(creation); err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}

	transitionId, err := h.manager.CreateWorkStateTransition(&creation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transitionId)
}
