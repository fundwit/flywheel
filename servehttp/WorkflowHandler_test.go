package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
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

var _ = Describe("WorkflowHandler", func() {
	var (
		router          *gin.Engine
		workflowManager *workflowManagerMock
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(servehttp.ErrorHandling())
		workflowManager = &workflowManagerMock{}
		servehttp.RegisterWorkflowHandler(router, workflowManager)
	})

	Describe("handleQueryWorkflows", func() {
		It("should return all workflows", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			Expect(err).To(BeNil())
			timeString := strings.Trim(string(timeBytes), `"`)
			workflowManager.QueryWorkflowsFunc =
				func(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error) {
					return &[]domain.Workflow{{ID: types.ID(10), Name: "test workflow", GroupID: types.ID(100),
						ThemeColor: "blue", ThemeIcon: "some-icon", CreateTime: t}}, nil
				}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"id": "10", "name": "test workflow", "themeColor":"blue", "themeIcon": "some-icon", "groupId": "100", "createTime": "` +
				timeString + `"}]`))
		})
		It("should be able to handle error when query workflows", func() {
			workflowManager.QueryWorkflowsFunc = func(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})
	Describe("handleCreateWorkflow", func() {
		It("should return 400 when failed to bind", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'WorkflowCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
				`Key: 'WorkflowCreation.GroupID' Error:Field validation for 'GroupID' failed on the 'required' tag\n` +
				`Key: 'WorkflowCreation.ThemeColor' Error:Field validation for 'ThemeColor' failed on the 'required' tag\n` +
				`Key: 'WorkflowCreation.ThemeIcon' Error:Field validation for 'ThemeIcon' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should be able to handle error when create workflow", func() {
			workflowManager.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error) {
				return nil, errors.New("a mocked error")
			}
			creation := buildDemoWorkflowCreation()
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should be able to create workflow successfully", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workflowManager.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error) {
				detail := domain.WorkflowDetail{
					Workflow:     domain.Workflow{ID: 123, Name: creation.Name, ThemeColor: "blue", ThemeIcon: "some-icon", GroupID: creation.GroupID, CreateTime: t},
					StateMachine: creation.StateMachine,
				}
				return &detail, nil
			}

			creation := buildDemoWorkflowCreation()
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test workflow", "themeColor": "blue", "themeIcon": "some-icon", "groupId": "333", "createTime": "` + timeString + `",
				"propertyDefinitions": null,
				"stateMachine": {
					"states": [{"name":"OPEN", "category": 1}, {"name":"CLOSED", "category": 2}],
					"transitions": [
						{"name": "done", "from": {"name":"OPEN", "category": 1}, "to": {"name":"CLOSED", "category": 2}},
						{"name": "reopen", "from": {"name":"CLOSED", "category": 2}, "to": {"name":"OPEN", "category": 1}}
					]
				}}`))
		})
	})

	Describe("handleDetailWorkflow", func() {
		It("should return specified workflow", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			Expect(err).To(BeNil())
			timeString := strings.Trim(string(timeBytes), `"`)
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ThemeColor: "blue", ThemeIcon: "some-icon", GroupID: types.ID(100), CreateTime: t},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "10", "name": "test workflow", "themeColor": "blue", "themeIcon": "some-icon", "groupId": "100", "createTime": "` + timeString + `",
				"propertyDefinitions":[{"name": "description"}, {"name": "creatorId"}],
				"stateMachine": {
					"states": [{"name":"PENDING", "category": 0}, {"name":"DOING", "category": 1}, {"name":"DONE", "category": 2}],
					"transitions": [
						{"name": "begin", "from": {"name":"PENDING", "category": 0}, "to": {"name":"DOING", "category": 1}},
						{"name": "close", "from": {"name":"PENDING", "category": 0}, "to": {"name":"DONE", "category": 2}},
						{"name": "cancel", "from": {"name":"DOING", "category": 1}, "to": {"name":"PENDING", "category": 0}},
						{"name": "finish", "from": {"name":"DOING", "category": 1}, "to": {"name":"DONE", "category": 2}},
						{"name": "reopen", "from": {"name":"DONE", "category": 2}, "to": {"name":"PENDING", "category": 0}}
					]
				}}`))
		})

		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return nil, domain.ErrNotFound
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle error when detail workflows", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleUpdateWorkflowsBase", func() {
		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when failed to bind", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'WorkflowBaseUpdation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
				`Key: 'WorkflowBaseUpdation.ThemeColor' Error:Field validation for 'ThemeColor' failed on the 'required' tag\n` +
				`Key: 'WorkflowBaseUpdation.ThemeIcon' Error:Field validation for 'ThemeIcon' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error) {
				return nil, domain.ErrNotFound
			}

			reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle error when detail workflows", func() {
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error) {
				return nil, errors.New("a mocked error")
			}

			reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should update workflow base info successfully when everything is ok", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			Expect(err).To(BeNil())
			timeString := strings.Trim(string(timeBytes), `"`)
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error) {
				return &domain.Workflow{ID: types.ID(10), Name: updating.Name, ThemeColor: updating.ThemeColor, ThemeIcon: updating.ThemeIcon,
					GroupID: types.ID(100), CreateTime: t}, nil
			}

			reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "10", "name": "updated works", "themeColor": "yellow", "themeIcon": "arrow",` +
				`"groupId": "100", "createTime": "` + timeString + `"}`))
		})
	})

	Describe("handleDeleteWorkflow", func() {
		It("should return 204 when workflow delete successfully", func() {
			workflowManager.DeleteWorkflowFunc = func(ID types.ID, sec *security.Context) error {
				return nil
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})

		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/abc", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should be able to handle error when delete workflow", func() {
			workflowManager.DeleteWorkflowFunc = func(ID types.ID, sec *security.Context) error {
				return errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleQueryTransitions", func() {
		It("should be able to query transitions with query: fromState", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", GroupID: types.ID(100), CreateTime: time.Now()},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/10/transitions?fromState=PENDING", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"begin","from":{"name": "PENDING", "category": 0},"to":{"name": "DOING", "category": 1}},
				{"name":"close","from":{"name": "PENDING", "category": 0},"to":{"name": "DONE", "category":2}}]`))
		})

		It("should be able to query transitions with query: fromState and toState", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", GroupID: types.ID(100), CreateTime: time.Now()},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/10/transitions?fromState=PENDING&toState=DONE", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"close","from":{"name": "PENDING", "category": 0},"to":{"name": "DONE", "category": 2}}]`))
		})

		It("should be able to query transitions with unknown state", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", GroupID: types.ID(100), CreateTime: time.Now()},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/10/transitions?fromState=UNKNOWN", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[]`))
		})

		It("should return 404 when workflow is not exists", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return nil, domain.ErrNotFound
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"the flow of id 2 was not found", "data": null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"strconv.ParseUint: parsing \"abc\": invalid syntax", "data": null}`))
		})

		It("should return 400 when validate failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/0/transitions?fromState=PENDING", nil)

			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", 
				"message":"Key: 'TransitionQuery.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag", "data": null}`))
		})
	})
})

func buildDemoWorkflowCreation() *flow.WorkflowCreation {
	return &flow.WorkflowCreation{Name: "test workflow", GroupID: types.ID(333), ThemeColor: "blue", ThemeIcon: "some-icon", StateMachine: state.StateMachine{
		States: []state.State{{Name: "OPEN", Category: state.InProcess}, {Name: "CLOSED", Category: state.Done}},
		Transitions: []state.Transition{
			{Name: "done", From: state.State{Name: "OPEN", Category: state.InProcess}, To: state.State{Name: "CLOSED", Category: state.Done}},
			{Name: "reopen", From: state.State{Name: "CLOSED", Category: state.Done}, To: state.State{Name: "OPEN", Category: state.InProcess}},
		},
	}}
}

type workflowManagerMock struct {
	CreateWorkStateTransitionFunc func(t *domain.WorkStateTransitionBrief, sec *security.Context) (*domain.WorkStateTransition, error)
	QueryWorkflowsFunc            func(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error)
	CreateWorkflowFunc            func(c *flow.WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error)
	DetailWorkflowFunc            func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error)
	UpdateWorkflowBaseFunc        func(ID types.ID, c *flow.WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error)
	DeleteWorkflowFunc            func(ID types.ID, sec *security.Context) error
}

func (m *workflowManagerMock) QueryWorkflows(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error) {
	return m.QueryWorkflowsFunc(query, sec)
}
func (m *workflowManagerMock) DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
	return m.DetailWorkflowFunc(ID, sec)
}
func (m *workflowManagerMock) CreateWorkflow(c *flow.WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error) {
	return m.CreateWorkflowFunc(c, sec)
}
func (m *workflowManagerMock) UpdateWorkflowBase(ID types.ID, c *flow.WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error) {
	return m.UpdateWorkflowBaseFunc(ID, c, sec)
}
func (m *workflowManagerMock) DeleteWorkflow(ID types.ID, sec *security.Context) error {
	return m.DeleteWorkflowFunc(ID, sec)
}
