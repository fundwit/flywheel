package servehttp

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func RegisterWorkStateTransitionHandler(r *gin.Engine, m work.WorkProcessManagerTraits, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/transitions", middleWares...)

	handler := &workStateTransitionHandler{manager: m, validator: validator.New()}
	g.POST("", handler.handleCreate)
}

type workStateTransitionHandler struct {
	manager   work.WorkProcessManagerTraits
	validator *validator.Validate
}

func (h *workStateTransitionHandler) handleCreate(c *gin.Context) {
	creation := domain.WorkStateTransitionBrief{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(creation); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	transitionId, err := h.manager.CreateWorkStateTransition(&creation, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, transitionId)
}
