package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/domain"
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
	)

	BeforeEach(func() {
		router = gin.Default()
		router.Use(servehttp.ErrorHandling())
		workManager = &workManagerMock{}
		servehttp.RegisterWorkHandler(router, workManager)
	})

	Describe("handleCreate", func() {
		It("should be able to serve create request", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
				detail := domain.WorkDetail{
					Work: domain.Work{
						ID:           123,
						Name:         creation.Name,
						GroupID:      creation.GroupID,
						FlowID:       domain.GenericWorkFlow.ID,
						OrderInState: t.UnixNano() / 1e6,
						CreateTime:   t,
						StateName:    domain.GenericWorkFlow.StateMachine.States[0].Name,
						State:        domain.GenericWorkFlow.StateMachine.States[0],
					},
					Type: domain.GenericWorkFlow.Workflow,
				}
				return &detail, nil
			}

			creation := domain.WorkCreation{Name: "test work", GroupID: types.ID(333)}
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(`{"id":"123","name":"test work","groupId":"333","flowId":"1", "orderInState": ` +
				strconv.FormatInt(t.UnixNano()/1e6, 10) + `, "createTime":"` + timeString + `",
				"stateName":"PENDING","type":{"id":"1","name":"GenericTask", "groupId": "0", "createTime": "2020-01-01T00:00:00Z"},"state":{"name":"PENDING","category":0},
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
			  "message": "Key: 'WorkCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\nKey: 'WorkCreation.GroupID' Error:Field validation for 'GroupID' failed on the 'required' tag",
			  "data": null
			}`))
		})

		It("should return 500 when service process failed", func() {
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`{"name":"test","groupId":"333"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleQuery", func() {
		It("should be able to serve query request", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workManager.QueryWorkFunc = func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
				works := []domain.Work{
					{ID: 1, Name: "work1", GroupID: types.ID(333), FlowID: 1, CreateTime: t, OrderInState: t.UnixNano() / 1e6, StateName: "PENDING", State: domain.StatePending},
					{ID: 2, Name: "work2", GroupID: types.ID(333), FlowID: 1, CreateTime: t, OrderInState: t.UnixNano() / 1e6, StateName: "PENDING", State: domain.StatePending},
				}
				return &works, nil
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/works?name=aaa", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"data":[{"id":"1","name":"work1","groupId":"333","flowId":"1",
				"createTime":"` + timeString + `","orderInState": ` + strconv.FormatInt(t.UnixNano()/1e6, 10) + ` ,
				"stateName":"PENDING","state":{"name":"PENDING", "category":0},
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null}, 
				{"id":"2","name":"work2","groupId":"333","flowId":"1", "orderInState": ` + strconv.FormatInt(t.UnixNano()/1e6, 10) + `,
				"createTime":"` + timeString + `","stateName":"PENDING","state":{"name":"PENDING", "category":0},
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null
				}],"total": 2}`))
		})

		It("should return 500 when service failed", func() {
			workManager.QueryWorkFunc = func(q *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
				{
					return &[]domain.Work{}, errors.New("a mocked error")
				}
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
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workManager.WorkDetailFunc = func(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
				return &domain.WorkDetail{
					Work: domain.Work{
						ID: 123, Name: "test work", GroupID: 100, CreateTime: t, FlowID: 1, OrderInState: 999,
						StateName: "DOING", State: domain.GenericWorkFlow.StateMachine.States[1],
						StateBeginTime: &t, ProcessBeginTime: &t, ProcessEndTime: &t,
					},
					Type: domain.GenericWorkFlow.Workflow,
				}, nil
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"123","name":"test work","groupId":"100","flowId":"1",
				"createTime":"` + timeString + `","orderInState": 999,
				"stateName":"DOING","state":{"name":"DOING", "category":1},
				"stateBeginTime": "` + timeString + `", "processBeginTime": "` + timeString + `", "processEndTime": "` + timeString + `",
				"type": {"id": "1", "name": "GenericTask", "groupId": "0", "createTime": "2020-01-01T00:00:00Z"}}`))
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
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workManager.UpdateWorkFunc = func(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
				return &domain.Work{ID: 100, Name: "new-name", GroupID: types.ID(333), CreateTime: t,
					FlowID: 1, OrderInState: t.UnixNano() / 1e6, StateName: "PENDING", State: domain.StatePending}, nil
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
				`{"name": "new-name"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"100","name":"new-name","stateName":"PENDING",
				"stateBeginTime": null, "processBeginTime": null, "processEndTime": null,
				"state":{"name":"PENDING", "category":0},"groupId":"333","flowId":"1","createTime":"` +
				timeString + `", "orderInState": ` + strconv.FormatInt(t.UnixNano()/1e6, 10) + `}`))
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
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"json: cannot unmarshal object into Go value of type []domain.StageRangeOrderUpdating","data":null}`))
		})
		It("should be able to handle process error", func() {
			workManager.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error {
				return errors.New("unexpected exception")
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/work-orders", bytes.NewReader([]byte(`[]`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
		})
		It("should be able to handle update", func() {
			workManager.UpdateStateRangeOrdersFunc = func(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error {
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
	UpdateStateRangeOrdersFunc func(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error
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
func (m *workManagerMock) UpdateStateRangeOrders(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error {
	return m.UpdateStateRangeOrdersFunc(wantedOrders, sec)
}
