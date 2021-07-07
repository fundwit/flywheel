package servehttp

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func RegisterWorkProcessStepHandler(r *gin.Engine, m work.WorkProcessManagerTraits, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/work-process-steps", middleWares...)
	handler := &workProcessStepHandler{manager: m, validator: validator.New()}
	g.GET("", handler.handleQuery)

	g.POST("", handler.handleCreate)

	t := r.Group("/v1/transitions", middleWares...)
	t.POST("", handler.handleCreate)
}

type workProcessStepHandler struct {
	manager   work.WorkProcessManagerTraits
	validator *validator.Validate
}

func (h *workProcessStepHandler) handleCreate(c *gin.Context) {
	creation := domain.WorkProcessStepCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(creation); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	err = h.manager.CreateWorkStateTransition(&creation, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusCreated)
}

func (h *workProcessStepHandler) handleQuery(c *gin.Context) {
	query := domain.WorkProcessStepQuery{}
	if err := c.ShouldBindWith(&query, binding.Query); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	works, err := h.manager.QueryProcessSteps(&query, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &misc.PagedBody{List: works, Total: uint64(len(*works))})
}
