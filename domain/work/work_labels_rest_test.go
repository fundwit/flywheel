package work_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain/work"
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

func TestCreateWorkLabelRelationAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	work.RegisterWorkLabelRelationsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, strings.NewReader("{}"))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'WorkLabelRelationReq.WorkId' Error:Field validation for 'WorkId' failed on the 'required' tag\n` +
			`Key: 'WorkLabelRelationReq.LabelId' Error:Field validation for 'LabelId' failed on the 'required' tag",
		"data":null}`))
		Expect(status).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, strings.NewReader(" \t "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, strings.NewReader(" xx "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "invalid character 'x' looking for beginning of value", "data": null}`))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		work.CreateWorkLabelRelationFunc = func(req work.WorkLabelRelationReq, c *session.Context) (*work.WorkLabelRelation, error) {
			return nil, errors.New("some error")
		}
		reqBody := `{"workId":"10", "labelId":"200"}`
		req := httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to create work label relation successfully", func(t *testing.T) {
		demoTime := types.TimestampOfDate(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := demoTime.Time().MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)

		work.CreateWorkLabelRelationFunc = func(req work.WorkLabelRelationReq, c *session.Context) (*work.WorkLabelRelation, error) {
			return &work.WorkLabelRelation{WorkId: req.WorkId, LabelId: req.LabelId, CreateTime: demoTime, CreatorId: 1000}, nil
		}
		reqBody := `{"workId":"100", "labelId":"2000"}`
		req := httptest.NewRequest(http.MethodPost, work.PathWorkLabelRelations, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"workId": "100", "creatorId": "1000", "createTime": "` + timeString +
			`", "labelId": "2000"}`))
	})
}

func TestDeleteWorkLabelRelationAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	work.RegisterWorkLabelRelationsRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, work.PathWorkLabelRelations, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'WorkLabelRelationReq.WorkId' Error:Field validation for 'WorkId' failed on the 'required' tag\n` +
			`Key: 'WorkLabelRelationReq.LabelId' Error:Field validation for 'LabelId' failed on the 'required' tag",
		"data":null}`))
	})

	t.Run("should be able to delete label", func(t *testing.T) {
		var r work.WorkLabelRelationReq
		work.DeleteWorkLabelRelationFunc = func(req work.WorkLabelRelationReq, c *session.Context) error {
			r = req
			return nil
		}
		req := httptest.NewRequest(http.MethodDelete, work.PathWorkLabelRelations+"?workId=100&labelId=2000", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeZero())

		Expect(r).To(Equal(work.WorkLabelRelationReq{WorkId: 100, LabelId: 2000}))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		var r work.WorkLabelRelationReq
		work.DeleteWorkLabelRelationFunc = func(req work.WorkLabelRelationReq, c *session.Context) error {
			r = req
			return errors.New("some error")
		}
		req := httptest.NewRequest(http.MethodDelete, work.PathWorkLabelRelations+"?workId=100&labelId=2000", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(r).To(Equal(work.WorkLabelRelationReq{WorkId: 100, LabelId: 2000}))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})
}
