package indices_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/indices"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestHandleIndexRequest(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	indices.RegisterIndicesRestAPI(router)

	t.Run("handle error", func(t *testing.T) {
		indices.ScheduleNewSyncRunFunc = func(sec *session.Context) (bool, error) {
			return false, errors.New("error on schedule new sync run")
		}
		req := httptest.NewRequest(http.MethodPost, indices.PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"error on schedule new sync run", "data":null}`))
	})
	t.Run("submit index request successfully", func(t *testing.T) {
		indices.ScheduleNewSyncRunFunc = func(sec *session.Context) (bool, error) {
			return true, nil
		}
		req := httptest.NewRequest(http.MethodPost, indices.PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"result": true}`))
	})
	t.Run("submit index request failed", func(t *testing.T) {
		indices.ScheduleNewSyncRunFunc = func(sec *session.Context) (bool, error) {
			return false, nil
		}
		req := httptest.NewRequest(http.MethodPost, indices.PathIndexRequests, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"result": false}`))
	})
}
