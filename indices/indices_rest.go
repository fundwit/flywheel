package indices

import (
	"flywheel/session"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var (
	PathIndexRequests        = "/v1/index-requests"
	PathPendingIndexRecovery = "/v1/pending-index-log-recovery"

	indexLogRecoveryLimiter = rate.NewLimiter(rate.Every(1*time.Minute), 1)
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
	if !indexLogRecoveryLimiter.Allow() {
		c.JSON(http.StatusOK, gin.H{"result": "request rate limited"})
		return
	}

	err := IndexlogRecoveryRoutineFunc(anonymousRecoveryInvoker)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, gin.H{"result": "started"})
}
