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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestQueryWorkflowsRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return all workflows", func(t *testing.T) {
		ts := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := ts.MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)
		flow.QueryWorkflowsFunc =
			func(query *domain.WorkflowQuery, s *session.Session) (*[]domain.Workflow, error) {
				return &[]domain.Workflow{{ID: types.ID(10), Name: "test workflow", ProjectID: types.ID(100),
					ThemeColor: "blue", ThemeIcon: "some-icon", CreateTime: ts}}, nil
			}

		req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`[{"id": "10", "name": "test workflow", "themeColor":"blue", "themeIcon": "some-icon", "projectId": "100", "createTime": "` +
			timeString + `"}]`))
	})

	t.Run("should be able to handle error when query workflows", func(t *testing.T) {
		flow.QueryWorkflowsFunc = func(query *domain.WorkflowQuery, s *session.Session) (*[]domain.Workflow, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})
}

func TestCreateWorkflowsRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when failed to bind", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
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

	t.Run("should be able to handle error when create workflow", func(t *testing.T) {
		flow.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, s *session.Session) (*domain.WorkflowDetail, error) {
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

	t.Run("should be able to create workflow successfully", func(t *testing.T) {
		ts := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := ts.MarshalJSON()
		timeString := strings.Trim(string(timeBytes), `"`)
		Expect(err).To(BeNil())
		flow.CreateWorkflowFunc = func(creation *flow.WorkflowCreation, s *session.Session) (*domain.WorkflowDetail, error) {
			detail := domain.WorkflowDetail{
				Workflow:     domain.Workflow{ID: 123, Name: creation.Name, ThemeColor: "blue", ThemeIcon: "some-icon", ProjectID: creation.ProjectID, CreateTime: ts},
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
}

func TestDetailWorkflowsRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return specified workflow", func(t *testing.T) {
		ts := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := ts.MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
			return &domain.WorkflowDetail{
				Workflow:            domain.Workflow{ID: types.ID(10), Name: "test workflow", ThemeColor: "blue", ThemeIcon: "some-icon", ProjectID: types.ID(100), CreateTime: ts},
				PropertyDefinitions: []domain.PropertyDefinition{{Name: "description"}, {Name: "creatorId"}},
				StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id": "10", "name": "test workflow", "themeColor": "blue", "themeIcon": "some-icon", "projectId": "100", "createTime": "` + timeString + `",
			"propertyDefinitions":[
				{"name": "description", "type":"", "title":"", "options":null},
				{"name": "creatorId",   "type":"", "title":"", "options":null}
			],
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

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
			return nil, bizerror.ErrNotFound
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should be able to handle error when detail workflows", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/1", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})
}

func TestUpdateWorkflowBaseRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when failed to bind", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
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

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, s *session.Session) (*domain.Workflow, error) {
			return nil, bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should be able to handle error when detail workflows", func(t *testing.T) {
		flow.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, s *session.Session) (*domain.Workflow, error) {
			return nil, errors.New("a mocked error")
		}

		reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should update workflow base info successfully when everything is ok", func(t *testing.T) {
		ts := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := ts.MarshalJSON()
		Expect(err).To(BeNil())
		timeString := strings.Trim(string(timeBytes), `"`)
		flow.UpdateWorkflowBaseFunc = func(ID types.ID, updating *flow.WorkflowBaseUpdation, s *session.Session) (*domain.Workflow, error) {
			return &domain.Workflow{ID: types.ID(10), Name: updating.Name, ThemeColor: updating.ThemeColor, ThemeIcon: updating.ThemeIcon,
				ProjectID: types.ID(100), CreateTime: ts}, nil
		}

		reqBody, err := json.Marshal(&flow.WorkflowBaseUpdation{Name: "updated works", ThemeColor: "yellow", ThemeIcon: "arrow"})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id": "10", "name": "updated works", "themeColor": "yellow", "themeIcon": "arrow",` +
			`"projectId": "100", "createTime": "` + timeString + `"}`))
	})
}

func TestDeleteWorkflowRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 204 when workflow delete successfully", func(t *testing.T) {
		flow.DeleteWorkflowFunc = func(ID types.ID, s *session.Session) error {
			return nil
		}
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should be able to handle error when delete workflow", func(t *testing.T) {
		flow.DeleteWorkflowFunc = func(ID types.ID, s *session.Session) error {
			return errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})
}

func TestQueryWorkflowTransitionsRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should be able to query transitions with query: fromState", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
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

	t.Run("should be able to query transitions with query: fromState and toState", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
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

	t.Run("should be able to query transitions with unknown state", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
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

	t.Run("should return 404 when workflow is not exists", func(t *testing.T) {
		flow.DetailWorkflowFunc = func(ID types.ID, s *session.Session) (*domain.WorkflowDetail, error) {
			return nil, bizerror.ErrNotFound
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/2/transitions?fromState=PENDING", nil)

		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"the flow of id 2 was not found", "data": null}`))
	})

	t.Run("should return 400 when bind failed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/abc/transitions?fromState=PENDING", nil)

		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param", "message":"strconv.ParseUint: parsing \"abc\": invalid syntax", "data": null}`))
	})

	t.Run("should return 400 when validate failed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/0/transitions?fromState=PENDING", nil)

		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param", 
			"message":"Key: 'TransitionQuery.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag", "data": null}`))
	})
}

func TestCreateWorkflowTransitionsRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/abc/transitions", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when request body is not json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when request body is json object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"code": "common.bad_param",
			"message": "json: cannot unmarshal object into Go value of type []state.Transition",
			"data": null
		}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`[{"name": "test"}]`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		fmt.Println(body)
		Expect(body).To(MatchJSON(`{
			"code":"common.bad_param",
			"message":"[0]: Key: 'Transition.From' Error:Field validation for 'From' failed on the 'required' tag\n` +
			`Key: 'Transition.To' Error:Field validation for 'To' failed on the 'required' tag",
			"data":null
		}`))
	})

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should return 400 state is unknown", func(t *testing.T) {
		flow.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return bizerror.ErrUnknownState
		}

		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"workflow.unknown_state","message":"unknown state","data":null}`))
	})

	t.Run("should be able to handle unexpected error", func(t *testing.T) {
		flow.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return errors.New("a mocked error")
		}
		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return 200 when everything is ok", func(t *testing.T) {
		flow.CreateWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return nil
		}

		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})
}

func TestDeleteWorkflowTransitionRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/abc/transitions", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when request body is not json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when request body is json object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"code": "common.bad_param",
			"message": "json: cannot unmarshal object into Go value of type []state.Transition",
			"data": null
		}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader([]byte(`[{"name": "test"}]`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		fmt.Println(body)
		Expect(body).To(MatchJSON(`{
			"code":"common.bad_param",
			"message":"[0]: Key: 'Transition.From' Error:Field validation for 'From' failed on the 'required' tag\n` +
			`Key: 'Transition.To' Error:Field validation for 'To' failed on the 'required' tag",
			"data":null}
		`))
	})

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should be able to handle unexpected error", func(t *testing.T) {
		flow.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return errors.New("a mocked error")
		}
		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return 200 when everything is ok", func(t *testing.T) {
		flow.DeleteWorkflowStateTransitionsFunc = func(id types.ID, transitions []state.Transition, s *session.Session) error {
			return nil
		}

		reqBody, err := json.Marshal(&[]state.Transition{{Name: "test", From: "PENDING", To: "DOING"}})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/1/transitions", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})
}

func TestUpdateWorkflowStateRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc/states", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when request body is not json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader([]byte(`{"name": "test"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"code": "common.bad_param",
			"message": "Key: 'WorkflowStateUpdating.OriginName' Error:Field validation for 'OriginName' failed on the 'required' tag",
			"data": null
		}`))
	})

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, s *session.Session) error {
			return bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should return 400 when new state is exist", func(t *testing.T) {
		flow.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, s *session.Session) error {
			return bizerror.ErrStateExisted
		}

		reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"workflow.state_existed","message":"state existed","data":null}`))
	})

	t.Run("should be able to handle unexpected error", func(t *testing.T) {
		flow.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, s *session.Session) error {
			return errors.New("a mocked error")
		}
		reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return 2xx when everything is ok", func(t *testing.T) {
		flow.UpdateWorkflowStateFunc = func(id types.ID, updating flow.WorkflowStateUpdating, s *session.Session) error {
			return nil
		}

		reqBody, err := json.Marshal(&flow.WorkflowStateUpdating{OriginName: "PENDING", Name: "QUEUED", Order: 2000})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})
}

func TestUpdateWorkflowStateOrdersRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/abc/state-orders", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when request body is not json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when request body is json object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"code": "common.bad_param",
			"message": "json: cannot unmarshal object into Go value of type []flow.StateOrderRangeUpdating",
			"data": null
		}`))
	})

	t.Run("should return 400 when failed to validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader([]byte(`[{}]`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"code": "common.bad_param",
			"message": "Key: 'StateOrderRangeUpdating.State' Error:Field validation for 'State' failed on the 'required' tag",
			"data": null
		}`))
	})

	t.Run("should return 404 when workflow is not exist", func(t *testing.T) {
		flow.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, s *session.Session) error {
			return bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should be able to handle unexpected error", func(t *testing.T) {
		flow.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, s *session.Session) error {
			return errors.New("a mocked error")
		}
		reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return 2xx when everything is ok", func(t *testing.T) {
		flow.UpdateStateRangeOrdersFunc = func(workflowID types.ID, wantedOrders *[]flow.StateOrderRangeUpdating, s *session.Session) error {
			return nil
		}
		reqBody, err := json.Marshal(&[]flow.StateOrderRangeUpdating{})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPut, "/v1/workflows/1/state-orders", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})
}

func TestCreateWorkflowStateRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should return 400 when id is invalid when creating state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/abc/states", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should return 400 when request body is not json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader([]byte(`bbb`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when failed to validate when creating state", func(t *testing.T) {
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

	t.Run("should return 404 when workflow is not exist when creating state", func(t *testing.T) {
		flow.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, s *session.Session) error {
			return bizerror.ErrNotFound
		}

		reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNotFound))
		Expect(body).To(MatchJSON(`{"code":"common.record_not_found","message":"record not found","data":null}`))
	})

	t.Run("should be able to handle unexpected error when creating state", func(t *testing.T) {
		flow.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, s *session.Session) error {
			return errors.New("a mocked error")
		}
		reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return 2xx when everything is ok when creating state", func(t *testing.T) {
		flow.CreateStateFunc = func(workflowID types.ID, creating *flow.StateCreating, s *session.Session) error {
			return nil
		}
		reqBody, err := json.Marshal(&flow.StateCreating{Name: "test", Category: 1, Order: 20001})
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/1/states", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeEmpty())
	})
}

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
	CreateWorkStateTransitionFunc func(t *domain.WorkProcessStepCreation, s *session.Session) error
}
