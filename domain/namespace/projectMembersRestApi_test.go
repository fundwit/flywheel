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

var _ = Describe("ProjectMembersRestApi", func() {
	var (
		router *gin.Engine
	)
	BeforeEach(func() {
		router = gin.Default()
		router.Use(bizerror.ErrorHandling())
		namespace.RegisterProjectMembersRestApis(router)
	})

	Describe("HandleQueryProjectMembers", func() {
		It("should be able to query project members successfully", func() {
			var palyload *domain.ProjectMemberQuery
			namespace.QueryProjectMembersFunc = func(d *domain.ProjectMemberQuery, sec *session.Context) (*[]domain.ProjectMemberDetail, error) {
				palyload = d
				t := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
				return &[]domain.ProjectMemberDetail{
					{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, CreateTime: t, Role: "guest"}, ProjectName: "p1", MemberName: "u10"},
				}, nil
			}

			req := httptest.NewRequest(http.MethodGet, namespace.ProjectsMemberApiRoot+"?projectId=123&memberId=456", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(palyload).ToNot(BeNil())
			Expect(*palyload.ProjectID).To(Equal(types.ID(123)))
			Expect(*palyload.MemberID).To(Equal(types.ID(456)))

			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(MatchJSON(`
				[{"projectId": "1", "memberId": "10", "role": "guest", "projectName": "p1", "memberName":"u10", "createTime": "2021-01-01T00:00:00Z"}]
			`))
		})
	})

	Describe("HandleCreateProjectMember", func() {
		It("should be able to create project member successfully", func() {
			var payload *domain.ProjectMemberCreation
			namespace.CreateProjectMemberFunc = func(c *domain.ProjectMemberCreation, sec *session.Context) error {
				payload = c
				return nil
			}

			req := httptest.NewRequest(http.MethodPost, namespace.ProjectsMemberApiRoot, common.StringReader(`
				{"projectId": "123", "memberId": "456", "role":"test"}
			`))
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeZero())

			Expect(*payload).To(Equal(domain.ProjectMemberCreation{ProjectID: 123, MemberId: 456, Role: "test"}))
		})
	})

	Describe("HandleDeleteProjectMember", func() {
		It("should be able to delete project member successfully", func() {
			var payload *domain.ProjectMemberDeletion
			namespace.DeleteProjectMemberFunc = func(d *domain.ProjectMemberDeletion, sec *session.Context) error {
				payload = d
				return nil
			}

			req := httptest.NewRequest(http.MethodDelete, namespace.ProjectsMemberApiRoot+"?projectId=123&memberId=456", nil)
			status, body, _ := testinfra.ExecuteRequest(req, router)
			Expect(body).To(BeZero())
			Expect(status).To(Equal(http.StatusOK))
			Expect(*payload).To(Equal(domain.ProjectMemberDeletion{ProjectID: 123, MemberID: 456}))
		})
	})
})
