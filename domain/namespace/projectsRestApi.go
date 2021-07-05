package namespace

import (
	"flywheel/domain"
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	ProjectsApiRoot = "/v1/projects"

	QueryProjectsFunc = QueryProjects
	CreateProjectFunc = CreateProject
	UpdateProjectFunc = UpdateProject
)

func RegisterProjectsRestApis(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	projects := r.Group(ProjectsApiRoot, middleWares...)
	projects.GET("", HandleQueryProjects)
	projects.POST("", HandleCreateProject)
	projects.PUT(":id", HandleUpdateProject)
}

func HandleQueryProjects(c *gin.Context) {
	result, err := QueryProjectsFunc(session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &result)
}

func HandleCreateProject(c *gin.Context) {
	payload := domain.ProjectCreating{}
	err := c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(err)
	}
	result, err := CreateProjectFunc(&payload, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, result)
}

func HandleUpdateProject(c *gin.Context) {
	id, err := misc.BindingPathID(c)
	if err != nil {
		panic(err)
	}

	payload := domain.ProjectUpdating{}
	err = c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(err)
	}
	err = UpdateProjectFunc(id, &payload, session.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
