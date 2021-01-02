package servehttp

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func RegisterWorkHandler(r *gin.Engine, m work.WorkManagerTraits) {
	// group: "", version: v1, resource: work
	g := r.Group("/v1/works")

	handler := &workHandler{workManager: m, validator: validator.New()}

	g.GET("", handler.handleQuery)
	g.POST("", handler.handleCreate)
	g.GET(":id", handler.handleDetail)
	//g.PUT("{id}", handler.handleUpdate)
	g.DELETE(":id", handler.handleDelete)

	// transition history
	//g.GET(":id/transitions", handler.)
	//g.POST(":id/transitions", handler.)
	//g.GET(":id/transitions/:tid", handler.)
}

type workHandler struct {
	workManager work.WorkManagerTraits
	validator   *validator.Validate
}

func (h *workHandler) handleCreate(c *gin.Context) {
	creation := domain.WorkCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}

	if err = h.validator.Struct(creation); err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}

	detail, err := h.workManager.CreateWork(&creation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, detail)
}
func (h *workHandler) handleDetail(c *gin.Context) {
	id, err := utils.ParseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("id") + "'"})
		return
	}
	detail, err := h.workManager.WorkDetail(id)
	if err != nil {
		// TODO 区分不存在和其他错误
		c.JSON(http.StatusNotFound, &ErrorBody{Code: "common.not_found", Message: ""})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *workHandler) handleQuery(c *gin.Context) {
	works, err := h.workManager.QueryWork()
	if err != nil {
		c.JSON(http.StatusInternalServerError, &ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, &PagedBody{List: works, Total: uint64(len(*works))})
}

func (h *workHandler) handleDelete(c *gin.Context) {
	id, err := utils.ParseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, &ErrorBody{Code: "common.bad_param", Message: "invalid id '" + c.Param("id") + "'"})
		return
	}

	err = h.workManager.DeleteWork(id)
	if err != nil {
		// TODO 区分不存在和其他错误
		c.JSON(http.StatusInternalServerError, &ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
