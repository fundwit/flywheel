package flow_test

import (
	"context"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/persistence"
	"flywheel/testinfra"
	"testing"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"

	"github.com/fundwit/go-commons/types"
)

func propertyDefinitionTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) {
	db := testinfra.StartMysqlTestDatabase("flywheel")
	err := db.DS.GormDB(context.Background()).AutoMigrate(
		&flow.WorkflowPropertyDefinition{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error
	Expect(err).To(BeNil())

	*testDatabase = db
	persistence.ActiveDataSourceManager = db.DS
}
func propertyDefinitionTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestCreatePropertyDefinition(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("workflow must be exist", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		pd, err := flow.CreatePropertyDefinition(404, domain.PropertyDefinition{},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(pd).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("only project manager has role", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: types.ID(1), StateMachine: creationDemo.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd, err := flow.CreatePropertyDefinition(workflow.ID, domain.PropertyDefinition{},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(pd).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))

		pd, err = flow.CreatePropertyDefinition(workflow.ID, domain.PropertyDefinition{},
			testinfra.BuildSecCtx(100, domain.ProjectRoleCommon+"_1"))
		Expect(pd).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should return created property definition and all data are persisted", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		workflow, err := flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd, err := flow.CreatePropertyDefinition(workflow.ID,
			domain.PropertyDefinition{Name: "testProperty", Type: "text", Title: "Test Property"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		Expect(pd.ID).ToNot(BeZero())
		Expect(pd.WorkflowID).To(Equal(workflow.ID))
		Expect(pd.PropertyDefinition).To(Equal(domain.PropertyDefinition{Name: "testProperty", Type: "text", Title: "Test Property"}))

		var properties []flow.WorkflowPropertyDefinition
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&flow.WorkflowPropertyDefinition{}).Scan(&properties).Error).To(BeNil())
		Expect(len(properties)).To(Equal(1))
		Expect(properties[0]).To(Equal(*pd))
	})

	t.Run("property name must be unique within workflow", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		workflow, err := flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd1, err := flow.CreatePropertyDefinition(workflow.ID,
			domain.PropertyDefinition{Name: "testProperty", Type: "text", Title: "Test Property"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(pd1).ToNot(BeNil())

		pd2, err := flow.CreatePropertyDefinition(workflow.ID,
			domain.PropertyDefinition{Name: "testProperty", Type: "text", Title: "Test Property2"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(pd2).To(BeNil())
		Expect(err.Error()).To(Equal(`Error 1062: Duplicate entry '` + workflow.ID.String() +
			`-testProperty' for key 'workflow_property_definitions.uni_workflow_prop'`))
	})
}

func TestQueryPropertyDefinitions(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("workflow must be exist", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		pd, err := flow.QueryPropertyDefinitions(404, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(pd).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("only project manager has role", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: types.ID(1), StateMachine: creationDemo.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd, err := flow.QueryPropertyDefinitions(workflow.ID, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(pd).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should query property definitions", func(t *testing.T) {
		defer propertyDefinitionTeardown(t, testDatabase)
		propertyDefinitionTestSetup(t, &testDatabase)

		workflow1, err := flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd1, err := flow.CreatePropertyDefinition(workflow1.ID,
			domain.PropertyDefinition{Name: "testProperty1", Type: "text", Title: "Test Property1"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		pd2, err := flow.CreatePropertyDefinition(workflow1.ID,
			domain.PropertyDefinition{Name: "testProperty2", Type: "text", Title: "Test Property2"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		properties, err := flow.QueryPropertyDefinitions(workflow1.ID, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(len(properties)).To(Equal(2))
		Expect(properties[0]).To(Equal(*pd1))
		Expect(properties[1]).To(Equal(*pd2))
	})
}
