package main

import (
	"flywheel/account"
	"flywheel/avatar"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/label"
	"flywheel/domain/namespace"
	"flywheel/domain/work"
	"flywheel/domain/workcontribution"
	"flywheel/es"
	"flywheel/event"
	"flywheel/indices"
	"flywheel/persistence"
	"flywheel/servehttp"
	"flywheel/session"
	"flywheel/sessions"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.Infoln("service start")

	dbConfig, err := persistence.ParseDatabaseConfigFromEnv()
	if err != nil {
		logrus.Fatalf("parse database config failed %v\n", err)
	}

	// create database (no conflict)
	if dbConfig.DriverType == "mysql" {
		if err := persistence.PrepareMysqlDatabase(dbConfig.DriverArgs); err != nil {
			logrus.Fatalf("failed to prepare database %v\n", err)
		}
	}

	// connect database
	ds := &persistence.DataSourceManager{DatabaseConfig: dbConfig}
	if err := ds.Start(); err != nil {
		logrus.Fatalf("database connection failed %v\n", err)
	}
	defer ds.Stop()
	persistence.ActiveDataSourceManager = ds

	// database migration (race condition)
	err = ds.GormDB().AutoMigrate(&domain.Work{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{},
		&workcontribution.WorkContributionRecord{}, &event.EventRecord{},
		&account.User{}, &domain.Project{}, &domain.ProjectMember{},
		&account.Role{}, &account.Permission{}, &label.Label{}, &work.WorkLabelRelation{},
		&account.UserRoleBinding{}, &account.RolePermissionBinding{}).Error
	if err != nil {
		logrus.Fatalf("database migration failed %v\n", err)
	}

	if err := account.DefaultSecurityConfiguration(); err != nil {
		logrus.Fatalf("failed to prepare default security configuration %v\n", err)
	}

	es.CreateClientFromEnv()

	engine := gin.Default()
	engine.Use(bizerror.ErrorHandling())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "flywheel")
	})

	securityMiddle := session.SimpleAuthFilter()

	sessions.RegisterSessionsHandler(engine)
	sessions.RegisterSessionHandler(engine, securityMiddle)
	indices.RegisterIndicesRestAPI(engine, securityMiddle)
	namespace.RegisterProjectsRestApis(engine, securityMiddle)
	namespace.RegisterProjectMembersRestApis(engine, securityMiddle)
	account.RegisterUsersHandler(engine, securityMiddle)

	label.RegisterLabelsRestAPI(engine, securityMiddle)
	work.RegisterWorkLabelRelationsRestAPI(engine, securityMiddle)
	label.LabelDeleteCheckFuncs = append(label.LabelDeleteCheckFuncs, work.IsLabelReferencedByWork)

	flow.DetailWorkflowFunc = flow.DetailWorkflow

	event.EventHandlers = append(event.EventHandlers, indices.IndexWorkEventHandle)

	servehttp.RegisterWorkflowHandler(engine, securityMiddle)

	servehttp.RegisterWorkHandler(engine, securityMiddle)

	servehttp.RegisterWorkProcessStepHandler(engine, securityMiddle)
	workcontribution.RegisterWorkContributionsHandlers(engine, securityMiddle)

	avatar.RegisterAvatarAPI(engine, securityMiddle)
	avatar.Bootstrap()

	err = engine.Run(":80")
	if err != nil {
		panic(err)
	}
}
