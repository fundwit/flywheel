package workrest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/label"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/domain/work/workrest"
	"flywheel/indices/search"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

var (
	router *gin.Engine

	demoTime         types.Timestamp
	timeString       string
	demoWorkflow     domain.WorkflowDetail
	demoWorkflowJson string
)

func beforeEach() {
	router = gin.Default()
	router.Use(bizerror.ErrorHandling())
	workrest.RegisterWorksRestAPI(router)

	demoTime = types.TimestampOfDate(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
	timeBytes, err := demoTime.Time().MarshalJSON()
	Expect(err).To(BeNil())
	timeString = strings.Trim(string(timeBytes), `"`)
	demoWorkflow = domain.WorkflowDetail{
		Workflow:            domain.Workflow{ID: 100, Name: "demo workflow", ThemeColor: "orange", ThemeIcon: "el-icon-star-on", ProjectID: 1000, CreateTime: demoTime.Time()},
		PropertyDefinitions: []domain.PropertyDefinition{{Name: "desc"}},
		StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
	}
	demoWorkflowJson = `{"id": "` + demoWorkflow.ID.String() + `", "name": "` + demoWorkflow.Name +
		`", "themeColor": "orange", "themeIcon": "el-icon-star-on", "projectId": "` +
		demoWorkflow.ProjectID.String() + `", "createTime": "` + timeString + `"}`
}

func TestCreateWorkAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to serve create request", func(t *testing.T) {
		beforeEach()

		work.CreateWorkFunc = func(creation *domain.WorkCreation, sec *session.Context) (*work.WorkDetail, error) {
			detail := work.WorkDetail{
				Work: domain.Work{
					ID:            123,
					Name:          creation.Name,
					Identifier:    "TEST-1",
					ProjectID:     creation.ProjectID,
					FlowID:        demoWorkflow.ID,
					OrderInState:  demoTime.Time().UnixNano() / 1e6,
					CreateTime:    demoTime,
					StateName:     demoWorkflow.StateMachine.States[0].Name,
					StateCategory: demoWorkflow.StateMachine.States[0].Category,
					State:         demoWorkflow.StateMachine.States[0],
				},
				Type:   &demoWorkflow.Workflow,
				Labels: []label.LabelBrief{{ID: 100, Name: "label100", ThemeColor: "red"}},
			}
			return &detail, nil
		}

		creation := domain.WorkCreation{Name: "test work", ProjectID: types.ID(333), FlowID: demoWorkflow.ID, InitialStateName: domain.StatePending.Name}
		reqBody, err := json.Marshal(creation)
		Expect(err).To(BeNil())
		req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusCreated))
		Expect(body).To(MatchJSON(`{"id":"123","name":"test work", "identifier":"TEST-1","projectId":"333","flowId":"` + demoWorkflow.ID.String() + `", "orderInState": ` +
			strconv.FormatInt(demoTime.Time().UnixNano()/1e6, 10) + `, "createTime":"` + timeString + `",
			"labels": [{"id":"100", "name":"label100", "themeColor":"red"}],
			"stateName":"PENDING", "stateCategory": 1, "type": ` + demoWorkflowJson + `,"state":{"name": "PENDING", "category": 1, "order": 1},
			"stateBeginTime": null,"processBeginTime":null, "processEndTime":null, "archivedTime": null}`))
	})

	t.Run("should return 400 when bind failed", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`bad json`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should return 400 when validate failed", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'WorkCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
			`Key: 'WorkCreation.ProjectID' Error:Field validation for 'ProjectID' failed on the 'required' tag\n` +
			`Key: 'WorkCreation.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag\n` +
			`Key: 'WorkCreation.InitialStateName' Error:Field validation for 'InitialStateName' failed on the 'required' tag",
			  "data": null
			}`))
	})

	t.Run("should return 500 when service process failed", func(t *testing.T) {
		beforeEach()

		work.CreateWorkFunc = func(creation *domain.WorkCreation, sec *session.Context) (*work.WorkDetail, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/works",
			bytes.NewReader([]byte(`{"name":"test","projectId":"333", "flowId": "1000", "initialStateName": "PENDING"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})
}

func TestQueryWorkAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to serve query request", func(t *testing.T) {
		beforeEach()

		search.SearchWorksFunc = func(q domain.WorkQuery, sec *session.Context) ([]work.WorkDetail, error) {
			works := []work.WorkDetail{
				{
					Work: domain.Work{ID: 1, Name: "work1", Identifier: "W-1", ProjectID: types.ID(333), FlowID: 1, CreateTime: demoTime, OrderInState: demoTime.Time().UnixNano() / 1e6,
						StateName: "PENDING", State: domain.StatePending, StateCategory: state.InBacklog},
				},
				{
					Work: domain.Work{ID: 2, Name: "work2", Identifier: "W-2", ProjectID: types.ID(333), FlowID: 1, CreateTime: demoTime, OrderInState: demoTime.Time().UnixNano() / 1e6,
						StateName: "DONE", State: domain.StateDone, StateCategory: state.Done},
				},
			}
			return works, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/works?name=aaa", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"data":[{"id":"1","name":"work1","identifier":"W-1","projectId":"333","flowId":"1",
			"createTime":"` + timeString + `","orderInState": ` + strconv.FormatInt(demoTime.Time().UnixNano()/1e6, 10) + ` ,
			"stateName":"PENDING", "stateCategory": 1, "state":{"name":"PENDING", "category":1, "order": 1},
			"stateBeginTime": null, "processBeginTime": null, "processEndTime": null, "archivedTime": null, "type":null, "labels":null }, 
			{"id":"2","name":"work2","identifier":"W-2","projectId":"333","flowId":"1", "orderInState": ` + strconv.FormatInt(demoTime.Time().UnixNano()/1e6, 10) + `,
			"createTime":"` + timeString + `","stateName":"DONE", "stateCategory": 3, "state":{"name":"DONE", "category":3, "order": 3},
			"stateBeginTime": null, "processBeginTime": null, "processEndTime": null, "archivedTime": null, "type":null, "labels":null 
			}],"total": 2}`))
	})

	t.Run("should be able to receive parameters", func(t *testing.T) {
		beforeEach()

		query := &domain.WorkQuery{}
		search.SearchWorksFunc = func(q domain.WorkQuery, sec *session.Context) ([]work.WorkDetail, error) {
			*query = q
			return []work.WorkDetail{}, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/works?name=aaa&projectId=3&stateCategory=2&stateCategory=3", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"data": [], "total": 0}`))
		Expect(query.Name).To(Equal("aaa"))
		Expect(query.ProjectID).To(Equal(types.ID(3)))
		Expect(query.StateCategories).To(Equal([]state.Category{state.InProcess, state.Done}))
	})

	t.Run("should return 500 when service failed", func(t *testing.T) {
		beforeEach()

		search.SearchWorksFunc = func(q domain.WorkQuery, sec *session.Context) ([]work.WorkDetail, error) {
			return []work.WorkDetail{}, errors.New("a mocked error")
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/works", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})
}

func TestDetailWorkAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should return 500 when service failed", func(t *testing.T) {
		beforeEach()

		work.DetailWorkFunc = func(id string, sec *session.Context) (*work.WorkDetail, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/works/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should return work detail as expected when everything is ok", func(t *testing.T) {
		beforeEach()

		work.DetailWorkFunc = func(id string, sec *session.Context) (*work.WorkDetail, error) {
			return &work.WorkDetail{
				Work: domain.Work{
					ID: 123, Name: "test work", Identifier: "W-1", ProjectID: 100, CreateTime: demoTime, FlowID: demoWorkflow.ID, OrderInState: 999,
					StateName: "DOING", StateCategory: demoWorkflow.StateMachine.States[1].Category, State: demoWorkflow.StateMachine.States[1],
					StateBeginTime: demoTime, ProcessBeginTime: demoTime, ProcessEndTime: demoTime,
				},
				Type:   &demoWorkflow.Workflow,
				Labels: []label.LabelBrief{{ID: 100, Name: "label100", ThemeColor: "red"}},
			}, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/works/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id":"123","name":"test work","identifier":"W-1", "projectId":"100","flowId":"` + demoWorkflow.ID.String() + `",
			"createTime":"` + timeString + `","orderInState": 999,
			"labels": [{"id":"100", "name":"label100", "themeColor":"red"}],
			"stateName":"DOING", "stateCategory": 2, "state":{"name":"DOING", "category":2, "order": 2},
			"stateBeginTime": "` + timeString + `", "processBeginTime": "` + timeString + `", "processEndTime": "` + timeString + `",
			"type": ` + demoWorkflowJson + `, "archivedTime": null}`))
	})
}

func TestUpdateWorkAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should failed when id is invalid", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodPut, "/v1/works/abc", bytes.NewReader([]byte(`bad json`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should failed when body bind failed", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(`bad json`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should failed when service failed", func(t *testing.T) {
		beforeEach()

		work.UpdateWorkFunc = func(id types.ID, u *domain.WorkUpdating, sec *session.Context) (*domain.Work, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
			`{"name": "new-name"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should be able to update work successfully", func(t *testing.T) {
		beforeEach()

		work.UpdateWorkFunc = func(id types.ID, u *domain.WorkUpdating, sec *session.Context) (*domain.Work, error) {
			return &domain.Work{ID: 100, Name: "new-name", Identifier: "W-1", ProjectID: types.ID(333), CreateTime: demoTime,
				FlowID: 1, OrderInState: demoTime.Time().UnixNano() / 1e6,
				StateName: "PENDING", StateCategory: domain.StatePending.Category, State: domain.StatePending}, nil
		}
		req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
			`{"name": "new-name"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`{"id":"100","name":"new-name","identifier":"W-1","stateName":"PENDING", "stateCategory": 1,
			"stateBeginTime": null, "processBeginTime": null, "processEndTime": null, "archivedTime": null,
			"state":{"name":"PENDING", "category":1, "order": 1},"projectId":"333","flowId":"1","createTime":"` +
			timeString + `", "orderInState": ` + strconv.FormatInt(demoTime.Time().UnixNano()/1e6, 10) + `}`))
	})
}

func TestDeleteWorkAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to handle delete work", func(t *testing.T) {
		beforeEach()

		work.DeleteWorkFunc = func(id types.ID, sec *session.Context) error {
			return nil
		}
		req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(len(body)).To(Equal(0))
	})

	t.Run("should be able to handle exception of invalid id", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodDelete, "/v1/works/abc", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
	})

	t.Run("should be able to handle exception of unexpected", func(t *testing.T) {
		beforeEach()

		work.DeleteWorkFunc = func(id types.ID, sec *session.Context) error {
			return errors.New("unexpected exception")
		}
		req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
	})
}

func TestCreateArchivedWorksAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to handle archive work", func(t *testing.T) {
		beforeEach()

		work.ArchiveWorksFunc = func(id []types.ID, sec *session.Context) error {
			return nil
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/archived-works", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"bad_request.body_not_found","message":"body not found","data":null}`))

		req = httptest.NewRequest(http.MethodPost, "/v1/archived-works", bytes.NewReader([]byte(`abc`)))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"bad_request.invalid_body_format","message":"invalid body format",
				"data":"invalid character 'a' looking for beginning of value"}`))

		req = httptest.NewRequest(http.MethodPost, "/v1/archived-works", bytes.NewReader([]byte(`{"workIdList": null}`)))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"bad_request.validation_failed","message":"validation failed",
			"data":"Key: 'WorkSelection.WorkIdList' Error:Field validation for 'WorkIdList' failed on the 'required' tag"}`))

		req = httptest.NewRequest(http.MethodPost, "/v1/archived-works", bytes.NewReader([]byte(`{"workIdList": []}`)))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"bad_request.validation_failed","message":"validation failed",
			"data":"Key: 'WorkSelection.WorkIdList' Error:Field validation for 'WorkIdList' failed on the 'gt' tag"}`))

		req = httptest.NewRequest(http.MethodPost, "/v1/archived-works", bytes.NewReader([]byte(`{"workIdList": ["111", 222]}`)))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(len(body)).To(Equal(0))
	})

	t.Run("should be able to handle exception of unexpected", func(t *testing.T) {
		beforeEach()

		work.ArchiveWorksFunc = func(id []types.ID, sec *session.Context) error {
			return errors.New("unexpected exception")
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/archived-works",
			bytes.NewReader([]byte(`{"workIdList": ["111", 222]}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
	})
}

func TestUpdateWorkOrdersAPI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to handle bind error", func(t *testing.T) {
		beforeEach()

		req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":
		"json: cannot unmarshal object into Go value of type []domain.WorkOrderRangeUpdating","data":null}`))
	})

	t.Run("should be able to handle process error", func(t *testing.T) {
		beforeEach()

		work.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *session.Context) error {
			return errors.New("unexpected exception")
		}
		req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`[]`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
	})

	t.Run("should be able to handle update", func(t *testing.T) {
		beforeEach()

		work.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *session.Context) error {
			return nil
		}
		req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`[]`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(BeEmpty())
	})
}
