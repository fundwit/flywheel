package servehttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/servehttp"
	"flywheel/utils"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
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
				{
					detail := domain.WorkDetail{
						Work: domain.Work{
							ID:         123,
							Name:       creation.Name,
							Group:      creation.Group,
							FlowID:     flow.GenericWorkFlow.ID,
							CreateTime: t,
							StateName:  flow.GenericWorkFlow.StateMachine.States[0].Name,
						},
						Type:  flow.GenericWorkFlow.WorkFlowBase,
						State: flow.GenericWorkFlow.StateMachine.States[0],
					}
					return &detail, nil
				}
			}

			creation := domain.WorkCreation{Name: "test work", Group: "test-group"}
			reqBody, err := json.Marshal(creation)
			Expect(err).To(BeNil())
			req := httptest.NewRequest(http.MethodPost, "/v1/works", bytes.NewReader(reqBody))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusCreated))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(string(bodyBytes)).To(MatchJSON(`{"id":"123","name":"test work","group":"test-group","flowId":"1",
			"createTime":"` + timeString + `","stateName":"PENDING","type":{"id":"1","name":"GenericTask"},"state":{"Name":"PENDING"}}`))
		})

		It("should return 400 when validate failed", func() {

		})

		It("should return 500 when service process failed", func() {

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
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(string(bodyBytes)).To(MatchJSON(`{"data":[{"id":"1","name":"work1","group":"default","flowId":"1",
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
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(string(bodyBytes)).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
		})
	})

	Describe("handleDetail", func() {

	})

	Describe("handleDelete", func() {
		It("should be able to handle delete work", func() {
			workManager.DeleteWorkFunc = func(id utils.ID) error {
				return nil
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusNoContent))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(len(bodyBytes)).To(Equal(0))
		})

		It("should be able to handle exception of invalid id", func() {
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/abc", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusBadRequest))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(string(bodyBytes)).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'abc'","data":null}`))
		})

		It("should be able to handle exception of unexpected", func() {
			workManager.DeleteWorkFunc = func(id utils.ID) error {
				return errors.New("unexpected exception")
			}
			req := httptest.NewRequest(http.MethodDelete, "/v1/works/123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			httpResponse := w.Result()
			defer httpResponse.Body.Close()

			Expect(httpResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			bodyBytes, _ := ioutil.ReadAll(httpResponse.Body)
			Expect(string(bodyBytes)).To(MatchJSON(`{"code":"common.internal_server_error","message":"unexpected exception","data":null}`))
		})
	})
})

type workManagerMock struct {
	QueryWorkFunc  func() (*[]domain.Work, error)
	WorkDetailFunc func(id utils.ID) (*domain.WorkDetail, error)
	CreateWorkFunc func(c *domain.WorkCreation) (*domain.WorkDetail, error)
	DeleteWorkFunc func(id utils.ID) error
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
func (m *workManagerMock) DeleteWork(id utils.ID) error {
	return m.DeleteWorkFunc(id)
}
