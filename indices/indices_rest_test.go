package indices

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
	"golang.org/x/time/rate"
)

func TestHandleIndexRequest(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	RegisterIndicesRestAPI(router)

	t.Run("handle error", func(t *testing.T) {
		ScheduleNewSyncRunFunc = func(sec *session.Session) (bool, error) {
			return false, errors.New("error on schedule new sync run")
		}
		req := httptest.NewRequest(http.MethodPost, PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"error on schedule new sync run", "data":null}`))
	})

	t.Run("submit index request successfully", func(t *testing.T) {
		ScheduleNewSyncRunFunc = func(sec *session.Session) (bool, error) {
			return true, nil
		}
		req := httptest.NewRequest(http.MethodPost, PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"result": true}`))
	})

	t.Run("submit index request failed", func(t *testing.T) {
		ScheduleNewSyncRunFunc = func(sec *session.Session) (bool, error) {
			return false, nil
		}
		req := httptest.NewRequest(http.MethodPost, PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"result": false}`))
	})
}

func TestHandleCreatePendingIndexLogsRecovery(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	RegisterIndicesRestAPI(router)

	t.Run("handle error", func(t *testing.T) {
		IndexlogRecoveryRoutineFunc = func(sec *session.Session) error {
			return errors.New("error on pending index log recovery")
		}
		req := httptest.NewRequest(http.MethodPost, PathPendingIndexRecovery, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"error on pending index log recovery", "data":null}`))
	})

	t.Run("create pending index log recovery routine successfully", func(t *testing.T) {
		indexLogRecoveryLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
		IndexlogRecoveryRoutineFunc = func(sec *session.Session) error {
			return nil
		}
		req := httptest.NewRequest(http.MethodPost, PathPendingIndexRecovery, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusCreated))
		Expect(body).To(MatchJSON(`{"result": "started"}`))

		req = httptest.NewRequest(http.MethodPost, PathPendingIndexRecovery, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"result": "request rate limited"}`))

		time.Sleep(101 * time.Millisecond)
		req = httptest.NewRequest(http.MethodPost, PathPendingIndexRecovery, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusCreated))
		Expect(body).To(MatchJSON(`{"result": "started"}`))
	})
}
