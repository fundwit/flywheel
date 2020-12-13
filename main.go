package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func main() {
	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	log.Println("service start")
	err := engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
