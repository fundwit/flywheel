package main

import (
	"flywheel/account"
	"flywheel/avatar"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/work"
	"flywheel/domain/workcontribution"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/servehttp"
	"flywheel/session"
	"flywheel/sessions"
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
		&workcontribution.WorkContributionRecord{}, &event.EventRecord{},
		&account.User{}, &domain.Project{}, &domain.ProjectMember{},
		&account.Role{}, &account.Permission{},
		&account.UserRoleBinding{}, &account.RolePermissionBinding{}).Error
	if err != nil {
		log.Fatalf("database migration failed %v\n", err)
	}

	if err := account.DefaultSecurityConfiguration(); err != nil {
		log.Fatalf("failed to prepare default security configuration %v\n", err)
	}

	engine := gin.Default()
	engine.Use(bizerror.ErrorHandling())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	securityMiddle := session.SimpleAuthFilter()

	sessions.RegisterSessionsHandler(engine)
	sessions.RegisterSessionHandler(engine, securityMiddle)

	namespace.RegisterProjectsRestApis(engine, securityMiddle)
	namespace.RegisterProjectMembersRestApis(engine, securityMiddle)
	account.RegisterUsersHandler(engine, securityMiddle)

	workflowManager := flow.NewWorkflowManager(ds)
	workProcessManager := work.NewWorkProcessManager(ds, workflowManager)
	servehttp.RegisterWorkflowHandler(engine, workflowManager, securityMiddle)

	servehttp.RegisterWorkHandler(engine, work.NewWorkManager(ds, workflowManager), securityMiddle)

	servehttp.RegisterWorkStateTransitionHandler(engine, workProcessManager, securityMiddle)
	servehttp.RegisterWorkProcessStepHandler(engine, workProcessManager, securityMiddle)
	workcontribution.RegisterWorkContributionsHandlers(engine, securityMiddle)

	avatar.RegisterAvatarAPI(engine, securityMiddle)
	avatar.Bootstrap()

	err = engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
