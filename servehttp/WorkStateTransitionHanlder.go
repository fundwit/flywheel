package servehttp

import (
	"flywheel/common"
	"flywheel/domain/flow"
	"flywheel/security"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func RegisterWorkStateTransitionHandler(r *gin.Engine, m flow.WorkflowManagerTraits, middleWares ...gin.HandlerFunc) {
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
		panic(&common.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(creation); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	transitionId, err := h.manager.CreateWorkStateTransition(&creation, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, transitionId)
}
