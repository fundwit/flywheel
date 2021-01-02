package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/domain"
	"flywheel/servehttp"
	"flywheel/testinfra"
	"flywheel/utils"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
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
		workManager = &workManagerMock{}
		servehttp.RegisterWorkHandler(router, workManager)
	})

	Describe("handleCreate", func() {
		It("should be able to serve create request", func() {
			t := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Now().Location())
			timeBytes, err := t.MarshalJSON()
			timeString := strings.Trim(string(timeBytes), `"`)
			Expect(err).To(BeNil())
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation) (*domain.WorkDetail, error) {
				detail := domain.WorkDetail{
					Work: domain.Work{
						ID:         123,
						Name:       creation.Name,
						Group:      creation.Group,
						FlowID:     domain.GenericWorkFlow.ID,
						CreateTime: t,
						StateName:  domain.GenericWorkFlow.StateMachine.States[0].Name,
					},
					Type:  domain.GenericWorkFlow.WorkFlowBase,
					State: domain.GenericWorkFlow.StateMachine.States[0],
				}
				return &detail, nil
			}

			creation := domain.WorkCreation{Name: "test work", Group: "test-group"}
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader(reqBody))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(`{"id":"123","name":"test work","group":"test-group","flowId":"1",
			"createTime":"` + timeString + `","stateName":"PENDING","type":{"id":"1","name":"GenericTask"},"state":{"name":"PENDING"}}`))
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
			Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"Key: 'WorkCreation.Name' Error:Field validation for 'Name' failed on the 'required' tag\nKey: 'WorkCreation.Group' Error:Field validation for 'Group' failed on the 'required' tag","data":null}`))
		})

		It("should return 500 when service process failed", func() {
			workManager.CreateWorkFunc = func(creation *domain.WorkCreation) (*domain.WorkDetail, error) {
				return nil, errors.New("a mocked error")
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader([]byte(`{"name":"test","group":"default"}`)))
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
			workManager.QueryWorkFunc = func() (*[]domain.Work, error) {
				{
					works := []domain.Work{
						{ID: 1, Name: "work1", Group: "default", FlowID: 1, CreateTime: t, StateName: "PENDING"},
						{ID: 2, Name: "work2", Group: "default", FlowID: 1, CreateTime: t, StateName: "PENDING"},
					}
					return &works, nil
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/works", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"data":[{"id":"1","name":"work1","group":"default","flowId":"1",
			"createTime":"` + timeString + `","stateName":"PENDING"},{"id":"2","name":"work2","group":"default","flowId":"1",
			"createTime":"` + timeString + `","stateName":"PENDING"}],"total": 2}`))
		})

		It("should return 500 when service failed", func() {
			workManager.QueryWorkFunc = func() (*[]domain.Work, error) {
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

	//Describe("handleDetail", func() {
	//
	//})

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
			workManager.UpdateWorkFunc = func(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error) {
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
			workManager.UpdateWorkFunc = func(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error) {
				return &domain.Work{ID: 100, Name: "new-name", Group: "default", FlowID: 1, CreateTime: t, StateName: "PENDING"}, nil
			}
			req := httptest.NewRequest(http.MethodPut, "/v1/works/100", bytes.NewReader([]byte(
				`{"name": "new-name"}`)))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`{"id":"100","name":"new-name","stateName":"PENDING","group":"default","flowId":"1","createTime":"` + timeString + `"}`))
		})
	})

	Describe("handleDelete", func() {
		It("should be able to handle delete work", func() {
			workManager.DeleteWorkFunc = func(id utils.ID) error {
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
			workManager.DeleteWorkFunc = func(id utils.ID) error {
				return errors.New("unexpected exception")
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
		})
	})
})

type workManagerMock struct {
	QueryWorkFunc  func() (*[]domain.Work, error)
	WorkDetailFunc func(id utils.ID) (*domain.WorkDetail, error)
	CreateWorkFunc func(c *domain.WorkCreation) (*domain.WorkDetail, error)
	DeleteWorkFunc func(id utils.ID) error
	UpdateWorkFunc func(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error)
}

func (m *workManagerMock) QueryWork() (*[]domain.Work, error) {
	return m.QueryWorkFunc()
}
func (m *workManagerMock) WorkDetail(id utils.ID) (*domain.WorkDetail, error) {
	return m.WorkDetailFunc(id)
}
func (m *workManagerMock) CreateWork(c *domain.WorkCreation) (*domain.WorkDetail, error) {
	return m.CreateWorkFunc(c)
}
func (m *workManagerMock) UpdateWork(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error) {
	return m.UpdateWorkFunc(id, u)
}
func (m *workManagerMock) DeleteWork(id utils.ID) error {
	return m.DeleteWorkFunc(id)
}
