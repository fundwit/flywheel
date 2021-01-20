package servehttp

import (
	"errors"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/security"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func RegisterWorkHandler(r *gin.Engine, m work.WorkManagerTraits, middleWares ...gin.HandlerFunc) {
	handler := &workHandler{workManager: m, validator: validator.New()}

	// group: "", version: v1, resource: work
	g := r.Group("/v1/works", middleWares...)
	g.GET("", handler.handleQuery)
	g.POST("", handler.handleCreate)
	//g.GET(":id", handler.handleDetail)
	g.PUT(":id", handler.handleUpdate)
	g.DELETE(":id", handler.handleDelete)

	o := r.Group("/v1/work-orders", middleWares...)
	o.PUT("", handler.handleUpdateOrders)
}

type workHandler struct {
	workManager work.WorkManagerTraits
	validator   *validator.Validate
}

func (h *workHandler) handleCreate(c *gin.Context) {
	creation := domain.WorkCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	if err = h.validator.Struct(creation); err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	detail, err := h.workManager.CreateWork(&creation, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, detail)
}

//func (h *workHandler) handleDetail(c *gin.Context) {
//	id, err := utils.ParseID(c.Param("id"))
//	if err != nil {
//		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("id") + "'"})
//		return
//	}
//	detail, err := h.workManager.WorkDetail(id)
//	if err != nil {
//		c.JSON(http.StatusNotFound, &ErrorBody{Code: "common.not_found", Message: ""})
//		return
//	}
//	c.JSON(http.StatusOK, detail)
//}

func (h *workHandler) handleQuery(c *gin.Context) {
	query := domain.WorkQuery{}
	_ = c.MustBindWith(&query, binding.Query)

	works, err := h.workManager.QueryWork(&query, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &common.PagedBody{List: works, Total: uint64(len(*works))})
}

func (h *workHandler) handleUpdate(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&common.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	updating := domain.WorkUpdating{}
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}

	updatedWork, err := h.workManager.UpdateWork(parsedId, &updating, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, updatedWork)
}

func (h *workHandler) handleUpdateOrders(c *gin.Context) {
	var updating []domain.StageRangeOrderUpdating
	err := c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&common.ErrBadParam{Cause: err})
	}
	err = h.workManager.UpdateStateRangeOrders(&updating, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusOK)
}

func (h *workHandler) handleDelete(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&common.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	err = h.workManager.DeleteWork(parsedId, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusNoContent, nil)
}
