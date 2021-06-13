package main

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/work"
	"flywheel/domain/workcontribution"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/servehttp"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
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
	persistence.ActiveDataSourceManager = ds

	// database migration (race condition)
	err = ds.GormDB().AutoMigrate(&domain.Work{}, &domain.WorkStateTransition{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{},
		&workcontribution.WorkContributionRecord{},
		&security.User{}, &domain.Project{}, &domain.ProjectMember{},
		&security.Role{}, &security.Permission{},
		&security.UserRoleBinding{}, &security.RolePermissionBinding{}).Error
	if err != nil {
		log.Fatalf("database migration failed %v\n", err)
	}

	if err := security.DefaultSecurityConfiguration(); err != nil {
		log.Fatalf("failed to prepare default security configuration %v\n", err)
	}

	engine := gin.Default()
	engine.Use(bizerror.ErrorHandling())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	security.RegisterSessionsHandler(engine)

	securityMiddle := security.SimpleAuthFilter()

	namespace.RegisterProjectsRestApis(engine, securityMiddle)
	namespace.RegisterProjectMembersRestApis(engine, securityMiddle)
	security.RegisterUsersHandler(engine, securityMiddle)
	security.RegisterSessionHandler(engine, securityMiddle)

	workflowManager := flow.NewWorkflowManager(ds)
	workProcessManager := work.NewWorkProcessManager(ds, workflowManager)
	servehttp.RegisterWorkflowHandler(engine, workflowManager, securityMiddle)

	servehttp.RegisterWorkHandler(engine, work.NewWorkManager(ds, workflowManager), securityMiddle)

	servehttp.RegisterWorkStateTransitionHandler(engine, workProcessManager, securityMiddle)
	servehttp.RegisterWorkProcessStepHandler(engine, workProcessManager, securityMiddle)
	workcontribution.RegisterWorkContributionsHandlers(engine, securityMiddle)

	err = engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
