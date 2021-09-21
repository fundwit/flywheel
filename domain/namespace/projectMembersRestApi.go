package namespace

import (
	"flywheel/domain"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	ProjectsMemberApiRoot = "/v1/project-members"

	QueryProjectMembersFunc = QueryProjectMemberDetails
	CreateProjectMemberFunc = CreateProjectMember
	DeleteProjectMemberFunc = DeleteProjectMember
)

func RegisterProjectMembersRestApis(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	projects := r.Group(ProjectsMemberApiRoot, middleWares...)
	projects.GET("", HandleQueryProjectMembers)
	projects.POST("", HandleCreateProjectMember)
	projects.DELETE("", HandleDeleteProjectMember)
}

func HandleQueryProjectMembers(c *gin.Context) {
	payload := domain.ProjectMemberQuery{}
	err := c.ShouldBindQuery(&payload)
	if err != nil {
		panic(err)
	}
	result, err := QueryProjectMembersFunc(&payload, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &result)
}

func HandleCreateProjectMember(c *gin.Context) {
	payload := domain.ProjectMemberCreation{}
	err := c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(err)
	}
	err = CreateProjectMemberFunc(&payload, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}

func HandleDeleteProjectMember(c *gin.Context) {
	payload := domain.ProjectMemberDeletion{}
	err := c.ShouldBindQuery(&payload)
	if err != nil {
		panic(err)
	}

	err = DeleteProjectMemberFunc(&payload, session.ExtractSessionFromGinContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
