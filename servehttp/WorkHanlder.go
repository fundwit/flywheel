package servehttp

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func RegisterWorkHandler(r *gin.Engine, m work.WorkManagerTraits, middleWares ...gin.HandlerFunc) {
	handler := &workHandler{workManager: m, validator: validator.New()}

	// group: "", version: v1, resource: work
	g := r.Group("/v1/works", middleWares...)
	g.GET("", handler.handleQuery)
	g.POST("", handler.handleCreate)
	g.GET(":id", handler.handleDetail)
	g.PUT(":id", handler.handleUpdate)
	g.DELETE(":id", handler.handleDelete)

	o := r.Group("/v1/work-orders", middleWares...)
	o.PUT("", handler.handleUpdateOrders)

	a := r.Group("/v1/archived-works", middleWares...)
	a.POST("", handler.handleCreateArchivedWorks)
}

type workHandler struct {
	workManager work.WorkManagerTraits
	validator   *validator.Validate
}

func (h *workHandler) handleCreate(c *gin.Context) {
	creation := domain.WorkCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(creation); err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	detail, err := h.workManager.CreateWork(&creation, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, detail)
}

func (h *workHandler) handleDetail(c *gin.Context) {
	detail, err := h.workManager.WorkDetail(c.Param("id"), session.FindSecurityContext(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *workHandler) handleQuery(c *gin.Context) {
	query := domain.WorkQuery{}
	_ = c.MustBindWith(&query, binding.Query)

	works, err := h.workManager.QueryWork(&query, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &misc.PagedBody{List: works, Total: uint64(len(*works))})
}

func (h *workHandler) handleUpdate(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	updating := domain.WorkUpdating{}
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	updatedWork, err := h.workManager.UpdateWork(parsedId, &updating, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, updatedWork)
}

func (h *workHandler) handleUpdateOrders(c *gin.Context) {
	var updating []domain.WorkOrderRangeUpdating
	err := c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	err = h.workManager.UpdateStateRangeOrders(&updating, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusOK)
}

func (h *workHandler) handleDelete(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	err = h.workManager.DeleteWork(parsedId, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusNoContent)
}

func (h *workHandler) handleCreateArchivedWorks(c *gin.Context) {
	query := domain.WorkSelection{}
	if err := c.ShouldBindBodyWith(&query, binding.JSON); err != nil {
		panic(err)
	}

	err := h.workManager.ArchiveWorks(query.WorkIdList, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusNoContent)
}
