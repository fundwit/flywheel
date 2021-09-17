package indices

import (
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	PathIndexRequests        = "/v1/index-requests"
	PathPendingIndexRecovery = "/v1/pending-index-log-recovery"
)

func RegisterIndicesRestAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathIndexRequests, middleWares...)
	g.POST("", handleIndexRequest)

	h := r.Group(PathPendingIndexRecovery, middleWares...)
	h.POST("", handleCreatePendingIndexLogsRecovery)
}

func handleIndexRequest(c *gin.Context) {
	success, err := ScheduleNewSyncRunFunc(session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, gin.H{"result": success})
}

func handleCreatePendingIndexLogsRecovery(c *gin.Context) {
	err := IndexlogRecoveryRoutineFunc(session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, gin.H{"result": "started"})
}
