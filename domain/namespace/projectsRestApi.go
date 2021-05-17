package namespace

import (
	"flywheel/common"
	"flywheel/domain"
	"flywheel/security"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"net/http"
)

var (
	ProjectsApiRoot = "/v1/projects"

	QueryProjectsFunc = QueryProjects
	CreateProjectFunc = CreateGroup
	UpdateProjectFunc = UpdateGroup
)

func RegisterNamespaceRestApis(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	projects := r.Group(ProjectsApiRoot, middleWares...)
	projects.GET("", HandleQueryProjects)
	projects.POST("", HandleCreateProject)
	projects.PUT(":id", HandleUpdateProject)
}

func HandleQueryProjects(c *gin.Context) {
	result, err := QueryProjectsFunc(security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, &result)
}

func HandleCreateProject(c *gin.Context) {
	payload := domain.GroupCreating{}
	err := c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(err)
	}
	result, err := CreateProjectFunc(&payload, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, result)
}

func HandleUpdateProject(c *gin.Context) {
	id, err := common.BindingPathID(c)
	if err != nil {
		panic(err)
	}

	payload := domain.GroupUpdating{}
	err = c.ShouldBindBodyWith(&payload, binding.JSON)
	if err != nil {
		panic(err)
	}
	err = UpdateProjectFunc(id, &payload, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
