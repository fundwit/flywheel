package main

import (
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/servehttp"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func main() {
	log.Println("service start")

	dbConfig, err := persistence.ParseDatabaseConfigFromEnv()
	if err != nil {
		log.Fatalf("parse database config failed %v\n", err)
	}

	// create database (no conflict)
	if dbConfig.DriverType == "mysql" {
		if err := persistence.PrepareMysqlDatabase(dbConfig.DriverArgs); err != nil {
			log.Fatalf("failed to prepare database %v\n", err)
		}
	}

	// connect database
	ds := &persistence.DataSourceManager{DatabaseConfig: dbConfig}
	if err := ds.Start(); err != nil {
		log.Fatalf("database conneciton failed %v\n", err)
	}
	defer ds.Stop()

	// database migration (race condition)
	err = ds.GormDB().AutoMigrate(&domain.Work{}).Error
	if err != nil {
		log.Fatalf("database migration failed %v\n", err)
	}

	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	workSvc := domain.NewWorkManager(ds)
	servehttp.RegisterWorkHandler(engine, workSvc)

	err = engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
