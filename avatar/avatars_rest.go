package avatar

import (
	"flywheel/misc"
	"flywheel/session"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	APIAccountAvatarsRoot = "/v1/account-avatars"
	DetailAvatarFunc      = DetailAvatar
	CreateAvatarFunc      = CreateAvatar
)

func RegisterAvatarAPI(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(APIAccountAvatarsRoot, middleWares...)
	g.GET(":id", HandleGetAvatar)
	g.POST(":id", HandleCreateAvatar)
}

func HandleGetAvatar(c *gin.Context) {
	id, err := misc.BindingPathID(c)
	if err != nil {
		panic(err)
	}

	bytes, err := DetailAvatarFunc(id, &session.Session{Context: c.Request.Context()})
	if err != nil {
		panic(err)
	}

	c.Data(http.StatusOK, "image/png", bytes)
}

func HandleCreateAvatar(c *gin.Context) {
	id, err := misc.BindingPathID(c)
	if err != nil {
		panic(err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		panic(err)
	}
	src, err := file.Open()
	if err != nil {
		panic(err)
	}
	defer src.Close()

	if err := CreateAvatarFunc(id, src, session.ExtractSessionFromGinContext(c)); err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, gin.H{})
}
