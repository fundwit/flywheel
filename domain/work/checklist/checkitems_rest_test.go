package checklist_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain/work/checklist"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestCreateCheckItemAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	checklist.RegisterCheckItemsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader("{}"))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'CheckItemCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
			`Key: 'CheckItemCreation.WorkId' Error:Field validation for 'WorkId' failed on the 'required' tag",
		"data":null}`))
		Expect(status).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader(" \t "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader(" xx "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "invalid character 'x' looking for beginning of value", "data": null}`))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		checklist.CreateCheckItemFunc = func(req checklist.CheckItemCreation, c *session.Context) (*checklist.CheckItem, error) {
			return nil, errors.New("some error")
		}

		reqBody := `{"workId":"10", "name":"test"}`
		req := httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to create work check item successfully", func(t *testing.T) {
		demoTime := types.TimestampOfDate(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := demoTime.Time().MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)

		checklist.CreateCheckItemFunc = func(req checklist.CheckItemCreation, c *session.Context) (*checklist.CheckItem, error) {
			return &checklist.CheckItem{ID: 1000, WorkId: req.WorkId, Name: req.Name,
				State: checklist.CheckItemStatePending, CreateTime: demoTime}, nil
		}

		reqBody := `{"workId":"100", "name":"test"}`
		req := httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id": "1000", "workId": "100", "name": "test", "state": "PENDING", "createTime": "` + timeString + `"}`))
	})
}

func TestDeleteCheckItemAPI(t *testing.T) {
	RegisterTestingT(t)
	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	checklist.RegisterCheckItemsRestAPI(router)

	t.Run("should be able to handle delete check item", func(t *testing.T) {
		checklist.DeleteCheckItemFunc = func(id types.ID, sec *session.Context) error {
			return nil
		}
		req := httptest.NewRequest(http.MethodDelete, checklist.PathCheckItems+"/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(len(body)).To(Equal(0))
	})

	t.Run("should be able to handle exception of invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, checklist.PathCheckItems+"/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))

		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should be able to handle exception of unexpected", func(t *testing.T) {
		checklist.DeleteCheckItemFunc = func(id types.ID, sec *session.Context) error {
			return errors.New("unexpected exception")
		}
		req := httptest.NewRequest(http.MethodDelete, checklist.PathCheckItems+"/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
	})
}
