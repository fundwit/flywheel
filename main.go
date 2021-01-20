package main

import (
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/work"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/servehttp"
	"flywheel/testinfra"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

func main() {
	log.Println("service start")

	dbConfig, err := persistence.ParseDatabaseConfigFromEnv()
	if err != nil {
		log.Fatalf("parse database config failed %v\n", err)
	}

	testDatabase := testinfra.StartMysqlTestDatabase("flywheel")
	err = testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}, &flow.WorkStateTransition{}).Error
	if err != nil {
		log.Fatalf("database migration failed %v\n", err)
	}
	defer testinfra.StopMysqlTestDatabase(testDatabase)
	dbConfig = testDatabase.DS.DatabaseConfig

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
	err = ds.GormDB().AutoMigrate(&domain.Work{}, &flow.WorkStateTransition{}, &security.User{}, &domain.Group{}, &domain.GroupMember{}).Error
	if err != nil {
		log.Fatalf("database migration failed %v\n", err)
	}

	ds.GormDB().Create(&security.User{ID: 1, Name: "admin", Secret: security.HashSha256("123456")})
	ds.GormDB().Create(&domain.Group{ID: 1, Name: "default", CreateTime: time.Now(), Creator: 1})
	ds.GormDB().Create(&domain.GroupMember{MemberId: 1, GroupID: 1, Role: "owner", CreateTime: time.Now()})

	engine := gin.Default()
	engine.Use(servehttp.ErrorHandling())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	security.DB = ds.GormDB()
	security.RegisterWorkHandler(engine)

	securityMiddle := security.SimpleAuthFilter()
	engine.GET("/me", securityMiddle, security.UserInfoQueryHandler)

	servehttp.RegisterWorkHandler(engine, work.NewWorkManager(ds), securityMiddle)
	servehttp.RegisterWorkflowHandler(engine, securityMiddle)
	servehttp.RegisterWorkStateTransitionHandler(engine, flow.NewWorkflowManager(ds), securityMiddle)

	err = engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
