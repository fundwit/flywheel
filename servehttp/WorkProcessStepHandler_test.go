package servehttp_test

import (
	"bytes"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/servehttp"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkProcessStepHandler", func() {
	var (
		router *gin.Engine
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		servehttp.RegisterWorkProcessStepHandler(router)
	})

	Describe("handleCreate", func() {
		It("should be able to handle bind error", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})
		It("should be able to handle validate error", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":
				"Key: 'WorkProcessStepCreation.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag\n` +
				`Key: 'WorkProcessStepCreation.WorkID' Error:Field validation for 'WorkID' failed on the 'required' tag\n` +
				`Key: 'WorkProcessStepCreation.FromState' Error:Field validation for 'FromState' failed on the 'required' tag\n` +
				`Key: 'WorkProcessStepCreation.ToState' Error:Field validation for 'ToState' failed on the 'required' tag","data":null}`))
		})
		It("should be able to handle service error", func() {
			work.CreateWorkStateTransitionFunc =
				func(c *domain.WorkProcessStepCreation, s *session.Session) error {
					return errors.New("a mocked error")
				}
			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(
				`{"flowId":1, "workId": "1", "fromState": "DONE", "toState": "DOING"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should be able to create transition", func() {
			work.CreateWorkStateTransitionFunc =
				func(c *domain.WorkProcessStepCreation, s *session.Session) error {
					return nil
				}

			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(
				`{"flowId":1, "workId": "100", "fromState": "PENDING", "toState": "DOING"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(BeZero())
		})
	})

	Describe("QueryProcessSteps", func() {
		It("should be able to handle bind error", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/work-process-steps?workId=bad", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"strconv.ParseUint: parsing \"bad\": invalid syntax","data":null}`))
		})

		It("should be able to handle validate error", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/work-process-steps", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"Key: 'WorkProcessStepQuery.WorkID' Error:Field validation for 'WorkID' failed on the 'required' tag","data":null}`))
		})

		It("should be able to handle service error", func() {
			work.QueryProcessStepsFunc =
				func(query *domain.WorkProcessStepQuery, s *session.Session) (*[]domain.WorkProcessStep, error) {
					return nil, errors.New("a mocked error")
				}
			req := httptest.NewRequest(http.MethodGet, "/v1/work-process-steps?workId=100", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should be able to query process steps", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			work.QueryProcessStepsFunc =
				func(query *domain.WorkProcessStepQuery, s *session.Session) (*[]domain.WorkProcessStep, error) {
					return &[]domain.WorkProcessStep{
						{WorkID: 100, FlowID: 1, StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category,
							NextStateName: domain.StateDoing.Name, NextStateCategory: domain.StateDoing.Category, CreatorID: 200, CreatorName: "user200",
							BeginTime: types.Timestamp(t), EndTime: types.Timestamp(t)},
						{WorkID: 100, FlowID: 1, StateName: domain.StateDoing.Name, StateCategory: domain.StateDoing.Category, BeginTime: types.Timestamp(t)},
					}, nil
				}

			req := httptest.NewRequest(http.MethodGet, "/v1/work-process-steps?workId=100", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"total": 2, "data": [
				{"workId": "100", "flowId": "1", "stateName": "PENDING", "stateCategory": 1, "nextStateName": "DOING", "nextStateCategory": 2, 
					"beginTime": "` + timeString + `", "endTime":"` + timeString + `", "creatorId": "200", "creatorName": "user200"},
				{"workId": "100", "flowId": "1", "stateName": "DOING", "stateCategory": 2, "beginTime": "` + timeString + `", "endTime": null,
					"nextStateName": "", "nextStateCategory": 0, "creatorId": "0", "creatorName": ""}
			]}`))
		})
	})
})
