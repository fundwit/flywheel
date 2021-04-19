package servehttp

import (
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/security"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func RegisterWorkProcessStepHandler(r *gin.Engine, m work.WorkProcessManagerTraits, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/work-process-steps", middleWares...)
	handler := &workProcessStepHandler{manager: m, validator: validator.New()}
	g.GET("", handler.handleQuery)
}

type workProcessStepHandler struct {
	manager   work.WorkProcessManagerTraits
	validator *validator.Validate
}

func (h *workProcessStepHandler) handleQuery(c *gin.Context) {
	query := domain.WorkProcessStepQuery{}
	if err := c.ShouldBindWith(&query, binding.Query); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	works, err := h.manager.QueryProcessSteps(&query, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &common.PagedBody{List: works, Total: uint64(len(*works))})
}
