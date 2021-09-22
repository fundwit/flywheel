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
		checklist.CreateCheckItemFunc = func(req checklist.CheckItemCreation, c *session.Session) (*checklist.CheckItem, error) {
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

		checklist.CreateCheckItemFunc = func(req checklist.CheckItemCreation, c *session.Session) (*checklist.CheckItem, error) {
			return &checklist.CheckItem{ID: 1000, WorkId: req.WorkId, Name: req.Name,
				Done: true, CreateTime: demoTime}, nil
		}

		reqBody := `{"workId":"100", "name":"test"}`
		req := httptest.NewRequest(http.MethodPost, checklist.PathCheckItems, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id": "1000", "workId": "100", "name": "test", "done": true, "createTime": "` + timeString + `"}`))
	})
}

func TestUpdateCheckItemAPI(t *testing.T) {
	RegisterTestingT(t)
	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	checklist.RegisterCheckItemsRestAPI(router)

	t.Run("should be able to handle update check item", func(t *testing.T) {
		var r checklist.CheckItemUpdate
		var cid types.ID
		checklist.UpdateCheckItemFunc = func(id types.ID, req checklist.CheckItemUpdate, c *session.Session) error {
			r = req
			cid = id
			return nil
		}

		// case1
		reqBody := `{"name":"aaa", "done": true}`
		req := httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(len(body)).To(Equal(0))

		Expect(cid).To(Equal(types.ID(123)))
		Expect(*r.Done).To(BeTrue())
		Expect(r).To(Equal(checklist.CheckItemUpdate{Name: "aaa", Done: r.Done}))

		// case2
		reqBody = `{"name":"aaa", "done": false}`
		req = httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(reqBody))
		status, _, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(*r.Done).To(BeFalse())

		reqBody = `{}`
		req = httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(reqBody))
		status, _, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(r.Done).To(BeNil())
		Expect(r.Name).To(BeZero())
	})

	t.Run("should be able to handle exception of invalid request", func(t *testing.T) {
		// case1
		req := httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))

		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))

		// case2
		req = httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"EOF", "data":null}`))

		// case3
		req = httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(" \t "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		// case4
		req = httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(" xx "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "invalid character 'x' looking for beginning of value", "data": null}`))
	})

	t.Run("should be able to handle exception of unexpected", func(t *testing.T) {
		checklist.UpdateCheckItemFunc = func(id types.ID, req checklist.CheckItemUpdate, c *session.Session) error {
			return errors.New("error on update check item")
		}

		reqBody := `{"name":"aaa", "done": false}`
		req := httptest.NewRequest(http.MethodPatch, checklist.PathCheckItems+"/123", strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"error on update check item","data":null}`))
	})
}

func TestDeleteCheckItemAPI(t *testing.T) {
	RegisterTestingT(t)
	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	checklist.RegisterCheckItemsRestAPI(router)

	t.Run("should be able to handle delete check item", func(t *testing.T) {
		checklist.DeleteCheckItemFunc = func(id types.ID, s *session.Session) error {
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
		checklist.DeleteCheckItemFunc = func(id types.ID, s *session.Session) error {
			return errors.New("unexpected exception")
		}
		req := httptest.NewRequest(http.MethodDelete, checklist.PathCheckItems+"/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
	})
}
