package indices

import (
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	PathIndexRequests = "/v1/index-request"
)

func RegisterIndicesRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathIndexRequests, middleWares...)
	g.POST("", handleIndexRequest)
}

func handleIndexRequest(c *gin.Context) {
	success, err := ScheduleNewSyncRunFunc(session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, gin.H{"result": success})
}
