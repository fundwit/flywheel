package servehttp_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/security"
	"flywheel/servehttp"
	"flywheel/testinfra"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

var _ = Describe("WorkProcessStepHandler", func() {
	var (
		router             *gin.Engine
		workProcessManager *workProcessManagerMock
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		workProcessManager = &workProcessManagerMock{}
		servehttp.RegisterWorkProcessStepHandler(router, workProcessManager)
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
			workProcessManager.QueryProcessStepsFunc =
				func(query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error) {
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
			workProcessManager.QueryProcessStepsFunc =
				func(query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error) {
					return &[]domain.WorkProcessStep{
						{WorkID: 100, FlowID: 1, StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category, BeginTime: t, EndTime: &t},
						{WorkID: 100, FlowID: 1, StateName: domain.StateDoing.Name, StateCategory: domain.StateDoing.Category, BeginTime: t},
					}, nil
				}

			req := httptest.NewRequest(http.MethodGet, "/v1/work-process-steps?workId=100", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"total": 2, "data": [
				{"workId": "100", "flowId": "1", "stateName": "PENDING", "stateCategory": 1, "beginTime": "` + timeString + `", "endTime":"` + timeString + `"},
				{"workId": "100", "flowId": "1", "stateName": "DOING", "stateCategory": 2, "beginTime": "` + timeString + `", "endTime": null}
			]}`))
		})
	})
})

type workProcessManagerMock struct {
	QueryProcessStepsFunc         func(query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error)
	CreateWorkStateTransitionFunc func(t *domain.WorkStateTransitionBrief, sec *security.Context) (*domain.WorkStateTransition, error)
}

func (m *workProcessManagerMock) QueryProcessSteps(
	query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error) {
	return m.QueryProcessStepsFunc(query, sec)
}
func (m *workProcessManagerMock) CreateWorkStateTransition(
	c *domain.WorkStateTransitionBrief, sec *security.Context) (*domain.WorkStateTransition, error) {
	return m.CreateWorkStateTransitionFunc(c, sec)
}
