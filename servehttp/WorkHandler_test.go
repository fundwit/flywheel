package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/domain"
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
	"strconv"
	"strings"
	"time"
)

var _ = Describe("WorkHandler", func() {
	var (
		router      *gin.Engine
		workManager *workManagerMock

		demoTime         time.Time
		timeString       string
		demoWorkflow     domain.WorkflowDetail
		demoWorkflowJson string
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(servehttp.ErrorHandling())
		workManager = &workManagerMock{}
		servehttp.RegisterWorkHandler(router, workManager)

		demoTime = time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
		timeBytes, err := demoTime.MarshalJSON()
		Expect(err).To(BeNil())
		timeString = strings.Trim(string(timeBytes), `"`)
		demoWorkflow = domain.WorkflowDetail{
			Workflow:            domain.Workflow{ID: 100, Name: "demo workflow", ThemeColor: "orange", ThemeIcon: "el-icon-star-on", GroupID: 1000, CreateTime: demoTime},
			PropertyDefinitions: []domain.PropertyDefinition{{Name: "desc"}},
			StateMachine:        domain.GenericWorkflowTemplate.StateMachine,
		}
		demoWorkflowJson = `{"id": "` + demoWorkflow.ID.String() + `", "name": "` + demoWorkflow.Name +
			`", "themeColor": "orange", "themeIcon": "el-icon-star-on", "groupId": "` +
			demoWorkflow.GroupID.String() + `", "createTime": "` + timeString + `"}`
	})

	Describe("handleCreate", func() {
		It("should be able to serve create request", func() {
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
				detail := domain.WorkDetail{
					Work: domain.Work{
						ID:            123,
						Name:          creation.Name,
						GroupID:       creation.GroupID,
						FlowID:        demoWorkflow.ID,
						OrderInState:  demoTime.UnixNano() / 1e6,
						CreateTime:    demoTime,
						StateName:     demoWorkflow.StateMachine.States[0].Name,
						StateCategory: demoWorkflow.StateMachine.States[0].Category,
						State:         demoWorkflow.StateMachine.States[0],
					},
					Type: demoWorkflow.Workflow,
				}
				return &detail, nil
			}

			creation := domain.WorkCreation{Name: "test work", GroupID: types.ID(333), FlowID: demoWorkflow.ID, InitialStateName: domain.StatePending.Name}
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(`{"id":"123","name":"test work","groupId":"333","flowId":"` + demoWorkflow.ID.String() + `", "orderInState": ` +
				strconv.FormatInt(demoTime.UnixNano()/1e6, 10) + `, "createTime":"` + timeString + `",
				"stateName":"PENDING", "stateCategory": 1, "type": ` + demoWorkflowJson + `,"state":{"name": "PENDING", "category": 1, "order": 1},
				"stateBeginTime": null,"processBeginTime":null, "processEndTime":null}`))
		})

		It("should return 400 when bind failed", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})
		It("should return 400 when validate failed", func() {
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{
			  "code": "common.bad_param",
			  "message": "Key: 'WorkCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
				`Key: 'WorkCreation.GroupID' Error:Field validation for 'GroupID' failed on the 'required' tag\n` +
				`Key: 'WorkCreation.FlowID' Error:Field validation for 'FlowID' failed on the 'required' tag\n` +
				`Key: 'WorkCreation.InitialStateName' Error:Field validation for 'InitialStateName' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 500 when service process failed", func() {
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/works",
				bytes.NewReader([]byte(`{"name":"test","groupId":"333", "flowId": "1000", "initialStateName": "PENDING"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleQuery", func() {
		It("should be able to serve query request", func() {
			workManager.QueryWorkFunc = func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
				works := []domain.Work{
					{ID: 1, Name: "work1", GroupID: types.ID(333), FlowID: 1, CreateTime: demoTime, OrderInState: demoTime.UnixNano() / 1e6,
						StateName: "PENDING", State: domain.StatePending, StateCategory: state.InBacklog},
					{ID: 2, Name: "work2", GroupID: types.ID(333), FlowID: 1, CreateTime: demoTime, OrderInState: demoTime.UnixNano() / 1e6,
						StateName: "DONE", State: domain.StateDone, StateCategory: state.Done},
				}
				return &works, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/works?name=aaa", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"data":[{"id":"1","name":"work1","groupId":"333","flowId":"1",
				"createTime":"` + timeString + `","orderInState": ` + strconv.FormatInt(demoTime.UnixNano()/1e6, 10) + ` ,
				"stateName":"PENDING", "stateCategory": 1, "state":{"name":"PENDING", "category":1, "order": 1},
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null}, 
				{"id":"2","name":"work2","groupId":"333","flowId":"1", "orderInState": ` + strconv.FormatInt(demoTime.UnixNano()/1e6, 10) + `,
				"createTime":"` + timeString + `","stateName":"DONE", "stateCategory": 3, "state":{"name":"DONE", "category":3, "order": 3},
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null
				}],"total": 2}`))
		})

		It("should be able to receive parameters", func() {
			query := &domain.WorkQuery{}
			workManager.QueryWorkFunc = func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
				query = q
				return &[]domain.Work{}, nil
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/works?name=aaa&groupId=3&stateCategory=2&stateCategory=3", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"data": [], "total": 0}`))
			Expect(query.Name).To(Equal("aaa"))
			Expect(query.GroupID).To(Equal(types.ID(3)))
			Expect(query.StateCategories).To(Equal([]state.Category{state.InProcess, state.Done}))
		})

		It("should return 500 when service failed", func() {
			workManager.QueryWorkFunc = func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
				return &[]domain.Work{}, errors.New("a mocked error")
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/works", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleDetail", func() {
		It("should failed when id is invalid", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/works/abc", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})
		It("should return 500 when service failed", func() {
			workManager.WorkDetailFunc = func(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})

		It("should return work detail as expected when everything is ok", func() {
			workManager.WorkDetailFunc = func(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
				return &domain.WorkDetail{
					Work: domain.Work{
						ID: 123, Name: "test work", GroupID: 100, CreateTime: demoTime, FlowID: demoWorkflow.ID, OrderInState: 999,
						StateName: "DOING", StateCategory: demoWorkflow.StateMachine.States[1].Category, State: demoWorkflow.StateMachine.States[1],
						StateBeginTime: &demoTime, ProcessBeginTime: &demoTime, ProcessEndTime: &demoTime,
					},
					Type: demoWorkflow.Workflow,
				}, nil
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"123","name":"test work","groupId":"100","flowId":"` + demoWorkflow.ID.String() + `",
				"createTime":"` + timeString + `","orderInState": 999,
				"stateName":"DOING", "stateCategory": 2, "state":{"name":"DOING", "category":2, "order": 2},
				"stateBeginTime": "` + timeString + `", "processBeginTime": "` + timeString + `", "processEndTime": "` + timeString + `",
				"type": ` + demoWorkflowJson + `}`))
		})
	})

	Describe("handleUpdate", func() {
		It("should failed when id is invalid", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/works/abc", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})
		It("should failed when body bind failed", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(`bad json`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
		})
		It("should failed when service failed", func() {
			workManager.UpdateWorkFunc = func(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
				`{"name": "new-name"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
		It("should be able to update work successfully", func() {
			workManager.UpdateWorkFunc = func(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
				return &domain.Work{ID: 100, Name: "new-name", GroupID: types.ID(333), CreateTime: demoTime,
					FlowID: 1, OrderInState: demoTime.UnixNano() / 1e6,
					StateName: "PENDING", StateCategory: domain.StatePending.Category, State: domain.StatePending}, nil
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
				`{"name": "new-name"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"100","name":"new-name","stateName":"PENDING", "stateCategory": 1,
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null,
				"state":{"name":"PENDING", "category":1, "order": 1},"groupId":"333","flowId":"1","createTime":"` +
				timeString + `", "orderInState": ` + strconv.FormatInt(demoTime.UnixNano()/1e6, 10) + `}`))
		})
	})

	Describe("handleDelete", func() {
		It("should be able to handle delete work", func() {
			workManager.DeleteWorkFunc = func(id types.ID, sec *security.Context) error {
				return nil
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusNoContent))
			Expect(len(body)).To(Equal(0))
		})

		It("should be able to handle exception of invalid id", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/abc", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should be able to handle exception of unexpected", func() {
			workManager.DeleteWorkFunc = func(id types.ID, sec *security.Context) error {
				return errors.New("unexpected exception")
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
		})
	})

	Describe("handleUpdateOrders", func() {
		It("should be able to handle bind error", func() {
			req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`{}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"json: cannot unmarshal object into Go value of type []domain.WorkOrderRangeUpdating","data":null}`))
		})
		It("should be able to handle process error", func() {
			workManager.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error {
				return errors.New("unexpected exception")
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`[]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
		})
		It("should be able to handle update", func() {
			workManager.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error {
				return nil
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`[]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeEmpty())
		})
	})
})

type workManagerMock struct {
	QueryWorkFunc              func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error)
	WorkDetailFunc             func(id types.ID, sec *security.Context) (*domain.WorkDetail, error)
	CreateWorkFunc             func(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error)
	DeleteWorkFunc             func(id types.ID, sec *security.Context) error
	UpdateWorkFunc             func(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error)
	UpdateStateRangeOrdersFunc func(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error
}

func (m *workManagerMock) QueryWork(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
	return m.QueryWorkFunc(q, sec)
}
func (m *workManagerMock) WorkDetail(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
	return m.WorkDetailFunc(id, sec)
}
func (m *workManagerMock) CreateWork(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
	return m.CreateWorkFunc(c, sec)
}
func (m *workManagerMock) UpdateWork(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
	return m.UpdateWorkFunc(id, u, sec)
}
func (m *workManagerMock) DeleteWork(id types.ID, sec *security.Context) error {
	return m.DeleteWorkFunc(id, sec)
}
func (m *workManagerMock) UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error {
	return m.UpdateStateRangeOrdersFunc(wantedOrders, sec)
}
