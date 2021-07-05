package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
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

var _ = Describe("WorkflowHandler", func() {
	var (
		router          *gin.Engine
		workflowManager *workflowManagerMock
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
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
				func(query *domain.WorkflowQuery, sec *session.Context) (*[]domain.Workflow, error) {
					return &[]domain.Workflow{{ID: types.ID(10), Name: "test workflow", ProjectID: types.ID(100),
						ThemeColor: "blue", ThemeIcon: "some-icon", CreateTime: t}}, nil
				}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"id": "10", "name": "test workflow", "themeColor":"blue", "themeIcon": "some-icon", "projectId": "100", "createTime": "` +
				timeString + `"}]`))
		})
		It("should be able to handle error when query workflows", func() {
			workflowManager.QueryWorkflowsFunc = func(query *domain.WorkflowQuery, sec *session.Context) (*[]domain.Workflow, error) {
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
				`Key: 'WorkflowCreation.ProjectID' Error:Field validation for 'ProjectID' failed on the 'required' tag\n` +
				`Key: 'WorkflowCreation.ThemeColor' Error:Field validation for 'ThemeColor' failed on the 'required' tag\n` +
				`Key: 'WorkflowCreation.ThemeIcon' Error:Field validation for 'ThemeIcon' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should be able to handle error when create workflow", func() {
			workflowManager.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, sec *session.Context) (*domain.WorkflowDetail, error) {
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
			workflowManager.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, sec *session.Context) (*domain.WorkflowDetail, error) {
				detail := domain.WorkflowDetail{
					Workflow:     domain.Workflow{ID: 123, Name: creation.Name, ThemeColor: "blue", ThemeIcon: "some-icon", ProjectID: creation.ProjectID, CreateTime: t},
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
			Expect(body).To(MatchJSON(`{"id": "123", "name": "test workflow", "themeColor": "blue", "themeIcon": "some-icon", "projectId": "333", "createTime": "` + timeString + `",
				"propertyDefinitions": null,
				"stateMachine": {
					"states": [{"name":"OPEN", "category": 2, "order": 10}, {"name":"CLOSED", "category": 3, "order": 20}],
					"transitions": [
						{"name": "done", "from": "OPEN", "to": "CLOSED"},
						{"name": "reopen", "from": "CLOSED", "to": "OPEN"}
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
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ThemeColor: "blue", ThemeIcon: "some-icon", ProjectID: types.ID(100), CreateTime: t},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "10", "name": "test workflow", "themeColor": "blue", "themeIcon": "some-icon", "projectId": "100", "createTime": "` + timeString + `",
				"propertyDefinitions":[{"name": "description"}, {"name": "creatorId"}],
				"stateMachine": {
					"states": [{"name":"PENDING", "category": 1, "order": 1}, {"name":"DOING", "category": 2, "order": 2},
								{"name":"DONE", "category": 3, "order": 3}],
					"transitions": [
						{"name": "begin", "from": "PENDING", "to": "DOING"},
						{"name": "close", "from": "PENDING", "to": "DONE"},
						{"name": "cancel", "from": "DOING", "to": "PENDING"},
						{"name": "finish", "from": "DOING", "to": "DONE"},
						{"name": "reopen", "from": "DONE", "to": "PENDING"}
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
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return nil, bizerror.ErrNotFound
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle error when detail workflows", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
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
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *session.Context) (*domain.Workflow, error) {
				return nil, bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle error when detail workflows", func() {
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *session.Context) (*domain.Workflow, error) {
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
			workflowManager.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, sec *session.Context) (*domain.Workflow, error) {
				return &domain.Workflow{ID: types.ID(10), Name: updating.Name, ThemeColor: updating.ThemeColor, ThemeIcon: updating.ThemeIcon,
					ProjectID: types.ID(100), CreateTime: t}, nil
			}

			reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id": "10", "name": "updated works", "themeColor": "yellow", "themeIcon": "arrow",` +
				`"projectId": "100", "createTime": "` + timeString + `"}`))
		})
	})

	Describe("handleDeleteWorkflow", func() {
		It("should return 204 when workflow delete successfully", func() {
			workflowManager.DeleteWorkflowFunc = func(ID types.ID, sec *session.Context) error {
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
			workflowManager.DeleteWorkflowFunc = func(ID types.ID, sec *session.Context) error {
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
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ProjectID: types.ID(100), CreateTime: time.Now()},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/10/transitions?fromState=PENDING", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"begin","from":"PENDING","to":"DOING"},
				{"name":"close","from":"PENDING","to":"DONE"}]`))
		})

		It("should be able to query transitions with query: fromState and toState", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ProjectID: types.ID(100), CreateTime: time.Now()},
					PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
					StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/workflows/10/transitions?fromState=PENDING&toState=DONE", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`[{"name":"close","from":"PENDING","to":"DONE"}]`))
		})

		It("should be able to query transitions with unknown state", func() {
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return &domain.WorkflowDetail{
					Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ProjectID: types.ID(100), CreateTime: time.Now()},
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
			workflowManager.DetailWorkflowFunc = func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
				return nil, bizerror.ErrNotFound
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

	Describe("handleCreateStateMachineTransitions", func() {
		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/abc/transitions", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when request body is not json", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when request body is json object", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "json: cannot unmarshal object into Go value of type []state.Transition",
			  "data": null
			}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`[{"name": "test"}]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'Transition.From' Error:Field validation for 'From' failed on the 'required' tag\n` +
				`Key: 'Transition.To' Error:Field validation for 'To' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should return 400 state is unknown", func() {
			workflowManager.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return bizerror.ErrUnknownState
			}

			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"workflow.unknown_state","message":"unknown state","data":null}`))
		})

		It("should be able to handle unexpected error", func() {
			workflowManager.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return errors.New("a mocked error")
			}
			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return 200 when everything is ok", func() {
			workflowManager.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return nil
			}

			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})
	})

	Describe("handleDeleteStateMachineTransitions", func() {
		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/abc/transitions", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when request body is not json", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when request body is json object", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "json: cannot unmarshal object into Go value of type []state.Transition",
			  "data": null
			}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`[{"name": "test"}]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'Transition.From' Error:Field validation for 'From' failed on the 'required' tag\n` +
				`Key: 'Transition.To' Error:Field validation for 'To' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle unexpected error", func() {
			workflowManager.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return errors.New("a mocked error")
			}
			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return 200 when everything is ok", func() {
			workflowManager.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, sec *session.Context) error {
				return nil
			}

			reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})
	})

	Describe("handleUpdateStateMachineState", func() {
		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when request body is not json", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader([]byte(`{"name": "test"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'WorkflowStateUpdating.OriginName' Error:Field validation for 'OriginName' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error {
				return bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should return 400 when new state is exist", func() {
			workflowManager.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error {
				return bizerror.ErrStateExisted
			}

			reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"workflow.state_existed","message":"state existed","data":null}`))
		})

		It("should be able to handle unexpected error", func() {
			workflowManager.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error {
				return errors.New("a mocked error")
			}
			reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return 2xx when everything is ok", func() {
			workflowManager.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error {
				return nil
			}

			reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})
	})

	Describe("handleUpdateStateMachineStateOrders", func() {
		It("should return 400 when id is invalid", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc/state-orders", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when request body is not json", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when request body is json object", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "json: cannot unmarshal object into Go value of type []flow.StateOrderRangeUpdating",
			  "data": null
			}`))
		})

		It("should return 400 when failed to validate", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`[{}]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'StateOrderRangeUpdating.State' Error:Field validation for 'State' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist", func() {
			workflowManager.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, sec *session.Context) error {
				return bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle unexpected error", func() {
			workflowManager.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, sec *session.Context) error {
				return errors.New("a mocked error")
			}
			reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return 2xx when everything is ok", func() {
			workflowManager.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, sec *session.Context) error {
				return nil
			}
			reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})
	})

	Describe("handleCreateStateMachineState", func() {
		It("should return 400 when id is invalid when creating state", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/abc/states", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should return 400 when request body is not json", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader([]byte(`bbb`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})

		It("should return 400 when failed to validate when creating state", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'StateCreating.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
				`Key: 'StateCreating.Category' Error:Field validation for 'Category' failed on the 'required' tag\n` +
				`Key: 'StateCreating.Order' Error:Field validation for 'Order' failed on the 'required' tag",
			  "data": null
			}`))

			// dive validate
			req = httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader([]byte(
				`{"name": "test", "category": 1, "order": 20001, "transitions": [{}]}`)))
			status, body, _ = testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'StateCreating.Transitions[0].Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
				`Key: 'StateCreating.Transitions[0].From' Error:Field validation for 'From' failed on the 'required' tag\n` +
				`Key: 'StateCreating.Transitions[0].To' Error:Field validation for 'To' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 404 when workflow is not exist when creating state", func() {
			workflowManager.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, sec *session.Context) error {
				return bizerror.ErrNotFound
			}

			reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
		})

		It("should be able to handle unexpected error when creating state", func() {
			workflowManager.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, sec *session.Context) error {
				return errors.New("a mocked error")
			}
			reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return 2xx when everything is ok when creating state", func() {
			workflowManager.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, sec *session.Context) error {
				return nil
			}
			reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(body).To(BeEmpty())
		})
	})
})

func buildDemoWorkflowCreation() *flow.WorkflowCreation {
	return &flow.WorkflowCreation{Name: "test workflow", ProjectID: types.ID(333), ThemeColor: "blue", ThemeIcon: "some-icon", StateMachine: state.StateMachine{
		States: []state.State{{Name: "OPEN", Category: state.InProcess, Order: 10}, {Name: "CLOSED", Category: state.Done, Order: 20}},
		Transitions: []state.Transition{
			{Name: "done", From: "OPEN", To: "CLOSED"},
			{Name: "reopen", From: "CLOSED", To: "OPEN"},
		},
	}}
}

type workflowManagerMock struct {
	CreateWorkStateTransitionFunc func(t *domain.WorkStateTransitionBrief, sec *session.Context) (*domain.WorkStateTransition, error)
	QueryWorkflowsFunc            func(query *domain.WorkflowQuery, sec *session.Context) (*[]domain.Workflow, error)
	CreateWorkflowFunc            func(c *flow.WorkflowCreation, sec *session.Context) (*domain.WorkflowDetail, error)
	DetailWorkflowFunc            func(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error)
	UpdateWorkflowBaseFunc        func(ID types.ID, c *flow.WorkflowBaseUpdation, sec *session.Context) (*domain.Workflow, error)
	DeleteWorkflowFunc            func(ID types.ID, sec *session.Context) error

	CreateWorkflowStateTransitionsFunc func(id types.ID, transitions []state.Transition, sec *session.Context) error
	DeleteWorkflowStateTransitionsFunc func(id types.ID, transitions []state.Transition, sec *session.Context) error

	CreateStateFunc            func(workflowID types.ID, creating *flow.StateCreating, sec *session.Context) error
	UpdateWorkflowStateFunc    func(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error
	UpdateStateRangeOrdersFunc func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, sec *session.Context) error
}

func (m *workflowManagerMock) QueryWorkflows(query *domain.WorkflowQuery, sec *session.Context) (*[]domain.Workflow, error) {
	return m.QueryWorkflowsFunc(query, sec)
}
func (m *workflowManagerMock) DetailWorkflow(ID types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
	return m.DetailWorkflowFunc(ID, sec)
}
func (m *workflowManagerMock) CreateWorkflow(c *flow.WorkflowCreation, sec *session.Context) (*domain.WorkflowDetail, error) {
	return m.CreateWorkflowFunc(c, sec)
}
func (m *workflowManagerMock) UpdateWorkflowBase(ID types.ID, c *flow.WorkflowBaseUpdation, sec *session.Context) (*domain.Workflow, error) {
	return m.UpdateWorkflowBaseFunc(ID, c, sec)
}
func (m *workflowManagerMock) DeleteWorkflow(ID types.ID, sec *session.Context) error {
	return m.DeleteWorkflowFunc(ID, sec)
}
func (m *workflowManagerMock) CreateWorkflowStateTransitions(id types.ID, transitions []state.Transition, sec *session.Context) error {
	return m.CreateWorkflowStateTransitionsFunc(id, transitions, sec)
}
func (m *workflowManagerMock) DeleteWorkflowStateTransitions(id types.ID, transitions []state.Transition, sec *session.Context) error {
	return m.DeleteWorkflowStateTransitionsFunc(id, transitions, sec)
}
func (m *workflowManagerMock) CreateState(workflowID types.ID, creating *flow.StateCreating, sec *session.Context) error {
	return m.CreateStateFunc(workflowID, creating, sec)
}
func (m *workflowManagerMock) UpdateWorkflowState(id types.ID, updating flow.WorkflowStateUpdating, sec *session.Context) error {
	return m.UpdateWorkflowStateFunc(id, updating, sec)
}
func (m *workflowManagerMock) UpdateStateRangeOrders(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, sec *session.Context) error {
	return m.UpdateStateRangeOrdersFunc(workflowID, wantedOrders, sec)
}
