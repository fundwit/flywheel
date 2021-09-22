package namespace_test

import (
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/namespace"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProjectRestApi", func() {
	var (
		router *gin.Engine
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		namespace.RegisterProjectsRestApis(router)
	})

	Describe("HandleQueryProjects", func() {
		It("should be able to query projects successfully", func() {
			namespace.QueryProjectsFunc = func(s *session.Session) (*[]domain.Project, error) {
				t := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
				return &[]domain.Project{{ID: 123, Identifier: "TED", Name: "test", NextWorkId: 10, CreateTime: t, Creator: 1}}, nil
			}

			req := httptest.NewRequest(http.MethodGet, namespace.ProjectsApiRoot, nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`
				[{"id": "123", "identifier": "TED", "name": "test", "nextWorkId": 10, "createTime": "2021-01-01T00:00:00Z", "creator": "1"}]
			`))
		})
	})

	Describe("HandleCreateProject", func() {
		It("should be able to create project successfully", func() {
			var payload *domain.ProjectCreating
			namespace.CreateProjectFunc = func(c *domain.ProjectCreating, s *session.Session) (*domain.Project, error) {
				payload = c
				t := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
				return &domain.Project{ID: 123, Identifier: c.Identifier, Name: c.Name, NextWorkId: 10, CreateTime: t, Creator: 100}, nil
			}

			req := httptest.NewRequest(http.MethodPost, namespace.ProjectsApiRoot, common.StringReader(`
				{"name": "test project", "identifier": "TEP"}
			`))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(body).To(MatchJSON(`
				{"id": "123", "identifier": "TEP", "name": "test project", "nextWorkId": 10, "createTime": "2021-01-01T00:00:00Z", "creator": "100"}
			`))
			Expect(status).To(Equal(http.StatusOK))

			Expect(*payload).To(Equal(domain.ProjectCreating{Name: "test project", Identifier: "TEP"}))
		})
	})

	Describe("HandleUpdateProject", func() {
		It("should be able to update project successfully", func() {
			var resId types.ID
			var payload *domain.ProjectUpdating
			namespace.UpdateProjectFunc = func(id types.ID, d *domain.ProjectUpdating, s *session.Session) error {
				resId = id
				payload = d
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, namespace.ProjectsApiRoot+"/123", common.StringReader(`
				{"name": "new project name"}
			`))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(body).To(BeZero())
			Expect(status).To(Equal(http.StatusOK))

			Expect(resId).To(Equal(types.ID(123)))
			Expect(*payload).To(Equal(domain.ProjectUpdating{Name: "new project name"}))
		})
	})
})
