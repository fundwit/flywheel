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
		It("should return workflows", func() {
			workflowManager.QueryWorkflowsFunc =
				func(sec *security.Context) (*[]domain.WorkflowDetail, error) {
					return &[]domain.WorkflowDetail{domain.GenericWorkFlow}, nil
				}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"id": "1", "name": "GenericTask", "groupId": "0", "createTime": "2020-01-01T00:00:00Z",
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
				}}]`))
		})
		It("should be able to handle error when query workflows", func() {
			workflowManager.QueryWorkflowsFunc = func(sec *security.Context) (*[]domain.WorkflowDetail, error) {
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
				`Key: 'WorkflowCreation.GroupID' Error:Field validation for 'GroupID' failed on the 'required' tag",
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
					Workflow:     domain.Workflow{ID: 123, Name: creation.Name, GroupID: creation.GroupID, CreateTime: t},
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
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test workflow", "groupId": "333", "createTime": "` + timeString + `",
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
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.GenericWorkFlow, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "1", "name": "GenericTask", "groupId": "0", "createTime": "2020-01-01T00:00:00Z",
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

	Describe("handleQueryStates", func() {
		It("should be able to query states success", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.GenericWorkFlow, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name": "PENDING","category":0},{"name": "DOING","category":1},{"name": "DONE","category":2}]`))
		})

		It("should return 404 when workflow is not exists", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return nil, domain.ErrNotFound
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"the flow of id 2 was not found", "data": null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"strconv.ParseUint: parsing \"abc\": invalid syntax", "data": null}`))
		})

		It("should return 400 when validate failed", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/0/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param", 
				"message":"invalid flowId '0'", "data": null}`))
		})
	})

	Describe("handleQueryTransitions", func() {
		It("should be able to query transitions with query: fromState", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.GenericWorkFlow, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=PENDING", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"begin","from":{"name": "PENDING", "category": 0},"to":{"name": "DOING", "category": 1}},
				{"name":"close","from":{"name": "PENDING", "category": 0},"to":{"name": "DONE", "category":2}}]`))
		})

		It("should be able to query transitions with query: fromState and toState", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.GenericWorkFlow, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=PENDING&toState=DONE", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"close","from":{"name": "PENDING", "category": 0},"to":{"name": "DONE", "category": 2}}]`))
		})

		It("should be able to query transitions with unknown state", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
				return &domain.GenericWorkFlow, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1/transitions?fromState=UNKNOWN", nil)
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
	return &flow.WorkflowCreation{Name: "test workflow", GroupID: types.ID(333), StateMachine: state.StateMachine{
		States: []state.State{{Name: "OPEN", Category: state.InProcess}, {Name: "CLOSED", Category: state.Done}},
		Transitions: []state.Transition{
			{Name: "done", From: state.State{Name: "OPEN", Category: state.InProcess}, To: state.State{Name: "CLOSED", Category: state.Done}},
			{Name: "reopen", From: state.State{Name: "CLOSED", Category: state.Done}, To: state.State{Name: "OPEN", Category: state.InProcess}},
		},
	}}
}
