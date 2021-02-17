package servehttp_test

import (
	"bytes"
	"errors"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/security"
	"flywheel/servehttp"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

var _ = Describe("WorkStateTransitionHandler", func() {
	var (
		router          *gin.Engine
		workflowManager *workflowManagerMock
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(servehttp.ErrorHandling())
		workflowManager = &workflowManagerMock{}
		servehttp.RegisterWorkStateTransitionHandler(router, workflowManager)
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
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"Key: 'WorkStateTransitionBrief.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag\nKey: 'WorkStateTransitionBrief.WorkID' Error:Field validation for 'WorkID' failed on the 'required' tag\nKey: 'WorkStateTransitionBrief.FromState' Error:Field validation for 'FromState' failed on the 'required' tag\nKey: 'WorkStateTransitionBrief.ToState' Error:Field validation for 'ToState' failed on the 'required' tag","data":null}`))
		})
		It("should be able to handle service error", func() {
			workflowManager.CreateWorkStateTransitionFunc =
				func(c *flow.WorkStateTransitionBrief, sec *security.Context) (*flow.WorkStateTransition, error) {
					return nil, errors.New("a mocked error")
				}
			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(
				`{"flowId":1, "workId": "1", "fromState": "DONE", "toState": "DOING"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should be able to create transition", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workflowManager.CreateWorkStateTransitionFunc =
				func(c *flow.WorkStateTransitionBrief, sec *security.Context) (*flow.WorkStateTransition, error) {
					return &flow.WorkStateTransition{ID: 123, Creator: types.ID(123), CreateTime: t, WorkStateTransitionBrief: flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 100, FromState: "PENDING", ToState: "DOING"}}, nil
				}

			req := httptest.NewRequest(http.MethodPost, "/v1/transitions", bytes.NewReader([]byte(
				`{"flowId":1, "workId": "100", "fromState": "PENDING", "toState": "DOING"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(`{"id": "123", "creator":"123", "createTime": "` + timeString +
				`", "flowId":"1", "workId":"100", "fromState":"PENDING", "toState": "DOING"}`))
		})
	})
})

type workflowManagerMock struct {
	CreateWorkStateTransitionFunc func(t *flow.WorkStateTransitionBrief, sec *security.Context) (*flow.WorkStateTransition, error)
	QueryWorkflowsFunc            func(sec *security.Context) (*[]domain.WorkFlow, error)
	DetailWorkflowFunc            func(ID types.ID, sec *security.Context) (*domain.WorkFlow, error)
}

func (m *workflowManagerMock) CreateWorkStateTransition(
	c *flow.WorkStateTransitionBrief, sec *security.Context) (*flow.WorkStateTransition, error) {
	return m.CreateWorkStateTransitionFunc(c, sec)
}
func (m *workflowManagerMock) QueryWorkflows(sec *security.Context) (*[]domain.WorkFlow, error) {
	return m.QueryWorkflowsFunc(sec)
}
func (m *workflowManagerMock) DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkFlow, error) {
	return m.DetailWorkflowFunc(ID, sec)
}
