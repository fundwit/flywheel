package workrest

import (
	"context"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/indices/search"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	PathWorks = "/v1/works"
)

func RegisterWorksRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group("/v1/works", middleWares...)
	g.GET("", handleQuery)
	g.POST("", handleCreate)
	g.GET(":id", handleDetail)
	g.PUT(":id", handleUpdate)
	g.DELETE(":id", handleDelete)

	o := r.Group("/v1/work-orders", middleWares...)
	o.PUT("", handleUpdateOrders)

	a := r.Group("/v1/archived-works", middleWares...)
	a.POST("", handleCreateArchivedWorks)
}

func handleQuery(c *gin.Context) {
	query := domain.WorkQuery{}
	err := c.MustBindWith(&query, binding.Query)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	//works, err := work.QueryWorkFunc(&query, session.FindSecurityContext(c))
	works, err := search.SearchWorksFunc(query, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &misc.PagedBody{List: works, Total: uint64(len(works))})
}

func handleCreate(c *gin.Context) {
	creation := domain.WorkCreation{}
	err := c.ShouldBindBodyWith(&creation, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	context.Background()
	detail, err := work.CreateWorkFunc(&creation, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, detail)
}

func handleDetail(c *gin.Context) {
	detail, err := work.DetailWorkFunc(c.Param("id"), session.ExtractSessionFromGinContext(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func handleUpdate(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	updating := domain.WorkUpdating{}
	err = c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}

	updatedWork, err := work.UpdateWorkFunc(parsedId, &updating, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, updatedWork)
}

func handleUpdateOrders(c *gin.Context) {
	var updating []domain.WorkOrderRangeUpdating
	err := c.ShouldBindBodyWith(&updating, binding.JSON)
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: err})
	}
	err = work.UpdateStateRangeOrdersFunc(&updating, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusOK)
}

func handleDelete(c *gin.Context) {
	parsedId, err := types.ParseID(c.Param("id"))
	if err != nil {
		panic(&bizerror.ErrBadParam{Cause: errors.New("invalid id '" + c.Param("id") + "'")})
	}

	err = work.DeleteWorkFunc(parsedId, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusNoContent)
}

func handleCreateArchivedWorks(c *gin.Context) {
	query := domain.WorkSelection{}
	if err := c.ShouldBindBodyWith(&query, binding.JSON); err != nil {
		panic(err)
	}

	err := work.ArchiveWorksFunc(query.WorkIdList, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.AbortWithStatus(http.StatusNoContent)
}
