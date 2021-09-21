package label_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain/label"
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

func TestQueryLabelsAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	label.RegisterLabelsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, label.PathLabels, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
			"message":"Key: 'LabelQuery.ProjectID' Error:Field validation for 'ProjectID' failed on the 'required' tag",
			"data":null}`))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		label.QueryLabelsFunc = func(q label.LabelQuery, ctx *session.Session) ([]label.Label, error) {
			return nil, errors.New("some error")
		}
		req := httptest.NewRequest(http.MethodGet, label.PathLabels+"?projectId=100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to handle query request successfully", func(t *testing.T) {
		demoTime := types.TimestampOfDate(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := demoTime.Time().MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)

		var q1 label.LabelQuery
		label.QueryLabelsFunc = func(q label.LabelQuery, ctx *session.Session) ([]label.Label, error) {
			q1 = q
			return []label.Label{{ID: 123, CreateTime: demoTime, CreatorID: 10, Name: "label123", ThemeColor: "red", ProjectID: 100}}, nil
		}
		req := httptest.NewRequest(http.MethodGet, label.PathLabels+"?projectId=100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`[{"id": "123", "creatorId":"10", "createTime": "` + timeString +
			`", "name": "label123", "themeColor":"red", "projectId": "100"}]`))
		Expect(q1).To(Equal(label.LabelQuery{ProjectID: 100}))
	})
}

func TestCreateLabelAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	label.RegisterLabelsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, label.PathLabels, strings.NewReader("{}"))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'LabelCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
			`Key: 'LabelCreation.ThemeColor' Error:Field validation for 'ThemeColor' failed on the 'required' tag\n` +
			`Key: 'LabelCreation.ProjectID' Error:Field validation for 'ProjectID' failed on the 'required' tag",
		"data":null}`))

		req = httptest.NewRequest(http.MethodPost, label.PathLabels, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, label.PathLabels, strings.NewReader(" \t "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, label.PathLabels, strings.NewReader(" xx "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "invalid character 'x' looking for beginning of value", "data": null}`))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		label.CreateLabelFunc = func(l label.LabelCreation, ctx *session.Session) (*label.Label, error) {
			return nil, errors.New("some error")
		}
		reqBody := `{"name":"test-label", "themeColor":"red", "projectId": "1234"}`
		req := httptest.NewRequest(http.MethodPost, label.PathLabels, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to create label successfully", func(t *testing.T) {
		demoTime := types.TimestampOfDate(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := demoTime.Time().MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)

		label.CreateLabelFunc = func(l label.LabelCreation, ctx *session.Session) (*label.Label, error) {
			return &label.Label{Name: l.Name, ThemeColor: l.ThemeColor, ProjectID: l.ProjectID, ID: 1111, CreatorID: 10, CreateTime: demoTime}, nil
		}
		reqBody := `{"name":"test-label", "themeColor":"red", "projectId": "999"}`
		req := httptest.NewRequest(http.MethodPost, label.PathLabels, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id": "1111", "creatorId": "10", "createTime": "` + timeString +
			`", "name": "test-label", "themeColor":"red", "projectId": "999"}`))
	})
}

func TestDeleteLabelAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	label.RegisterLabelsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, label.PathLabels+"/aaa", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "invalid id 'aaa'",
		"data":null}`))
	})

	t.Run("should be able to delete label", func(t *testing.T) {
		var reqId types.ID
		label.DeleteLabelFunc = func(id types.ID, ctx *session.Session) error {
			reqId = id
			return nil
		}
		req := httptest.NewRequest(http.MethodDelete, label.PathLabels+"/100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeZero())

		Expect(reqId).To(Equal(types.ID(100)))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		label.DeleteLabelFunc = func(id types.ID, ctx *session.Session) error {
			return errors.New("some error")
		}
		req := httptest.NewRequest(http.MethodDelete, label.PathLabels+"/100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})
}
