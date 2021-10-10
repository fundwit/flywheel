package flow_test

import (
	"context"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"

	"github.com/fundwit/go-commons/types"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T, testDatabase **testinfra.TestDatabase) {
	db := testinfra.StartMysqlTestDatabase("flywheel")
	assert.Nil(t, db.DS.GormDB(context.Background()).AutoMigrate(&domain.Work{}, &domain.WorkProcessStep{},
		&flow.WorkflowPropertyDefinition{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error)
	persistence.ActiveDataSourceManager = db.DS
	*testDatabase = db
}
func teardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

var creationDemo = &flow.WorkflowCreation{Name: "test workflow", ProjectID: types.ID(1), ThemeColor: "blue", ThemeIcon: "some-icon", StateMachine: state.StateMachine{
	States: []state.State{{Name: "OPEN", Category: state.InProcess}, {Name: "CLOSED", Category: state.Done}},
	Transitions: []state.Transition{
		{Name: "done", From: "OPEN", To: "CLOSED"},
		{Name: "reopen", From: "CLOSED", To: "OPEN"},
	},
}}

func TestCreateWorkflow(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should forbid to create to other project", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: types.ID(1), StateMachine: creationDemo.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(workflow).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should catch database errors", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		_, err := flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		_, err = flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		_, err = flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})

	t.Run("should return created workflow and all data are persisted", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		workflow, err := flow.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(workflow.Name).To(Equal(creationDemo.Name))
		Expect(workflow.ThemeColor).To(Equal("blue"))
		Expect(workflow.ThemeIcon).To(Equal("some-icon"))
		Expect(workflow.ProjectID).To(Equal(creationDemo.ProjectID))
		Expect(workflow.StateMachine).To(Equal(creationDemo.StateMachine))
		Expect(workflow.ID).ToNot(BeNil())
		Expect(workflow.CreateTime).ToNot(BeNil())
		workflow.CreateTime = workflow.CreateTime.Round(time.Millisecond)

		var flows []domain.Workflow
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.Workflow{}).Scan(&flows).Error).To(BeNil())
		Expect(len(flows)).To(Equal(1))
		Expect(flows[0]).To(Equal(workflow.Workflow))

		var states []domain.WorkflowState
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowState{}).Order("`order` ASC").Scan(&states).Error).To(BeNil())
		Expect(len(states)).To(Equal(2))
		Expect(states[0].WorkflowID).To(Equal(workflow.ID))
		Expect(states[0].CreateTime).To(Equal(workflow.CreateTime))
		Expect(states[0].Order).To(Equal(10001))
		Expect(state.State{Name: states[0].Name, Category: states[0].Category, Order: states[0].Order}).To(Equal(workflow.StateMachine.States[0]))

		Expect(states[1].WorkflowID).To(Equal(workflow.ID))
		Expect(states[1].CreateTime).To(Equal(workflow.CreateTime))
		Expect(states[1].Order).To(Equal(10002))
		Expect(state.State{Name: states[1].Name, Category: states[1].Category, Order: states[1].Order}).To(Equal(workflow.StateMachine.States[1]))

		var transitions []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowStateTransition{}).Scan(&transitions).Error).To(BeNil())
		Expect(len(transitions)).To(Equal(2))
		Expect(transitions[0].WorkflowID).To(Equal(workflow.ID))
		Expect(transitions[0].CreateTime).To(Equal(workflow.CreateTime))
		Expect(transitions[0].Name).To(Equal(workflow.StateMachine.Transitions[1].Name))
		Expect(transitions[0].FromState).To(Equal(workflow.StateMachine.Transitions[1].From))
		Expect(transitions[0].ToState).To(Equal(workflow.StateMachine.Transitions[1].To))

		Expect(transitions[1].WorkflowID).To(Equal(workflow.ID))
		Expect(transitions[1].CreateTime).To(Equal(workflow.CreateTime))
		Expect(transitions[1].Name).To(Equal(workflow.StateMachine.Transitions[0].Name))
		Expect(transitions[1].FromState).To(Equal(workflow.StateMachine.Transitions[0].From))
		Expect(transitions[1].ToState).To(Equal(workflow.StateMachine.Transitions[0].To))
	})
}

func TestDetailWorkflow(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should forbid to get workflow detail with permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		detail, err := flow.DetailWorkflow(workflow.ID, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2"))
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		detail, err := flow.DetailWorkflow(404, testinfra.BuildSecCtx(123))
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
		Expect(detail).To(BeNil())
	})

	t.Run("should be able to return workflow detail if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())

		detail, err := flow.DetailWorkflow(workflow.ID, testinfra.BuildSecCtx(123, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())
		Expect(detail.ID).ToNot(BeNil())
		Expect(detail.Name).To(Equal("test work"))
		Expect(detail.ThemeColor).To(Equal("blue"))
		Expect(detail.ThemeIcon).To(Equal("foo"))
		Expect(detail.ProjectID).To(Equal(types.ID(333)))
		Expect(detail.CreateTime).ToNot(BeNil())
		Expect(detail.StateMachine.States).To(Equal(domain.GenericWorkflowTemplate.StateMachine.States))
		Expect(state.SortTransitions(detail.StateMachine.Transitions)).To(
			Equal(state.SortTransitions(domain.GenericWorkflowTemplate.StateMachine.Transitions)))
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		detail, err := flow.DetailWorkflow(workflow.ID, sec)
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		detail, err = flow.DetailWorkflow(workflow.ID, sec)
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		detail, err = flow.DetailWorkflow(workflow.ID, sec)
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestDeleteWorkflow(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		err := flow.DeleteWorkflow(404, testinfra.BuildSecCtx(123))
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should forbid to delete workflow without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		// case 1: without any permission
		err = flow.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should forbid to delete workflow if it still be referenced by work", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).Save(&domain.Work{ID: 1, Name: "test", ProjectID: 100, CreateTime: types.CurrentTimestamp(), FlowID: workflow.ID,
			OrderInState: 1, StateName: "PENDING", StateCategory: 0,
			StateBeginTime: types.Timestamp{}, ProcessBeginTime: types.Timestamp{}, ProcessEndTime: types.Timestamp{}})

		err = flow.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_1"))
		Expect(err).To(Equal(bizerror.ErrWorkflowIsReferenced))
	})

	t.Run("should forbid to delete workflow if it still be referenced by workProcessStep", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).Save(&domain.WorkProcessStep{WorkID: 1, FlowID: workflow.ID, StateName: "PENDING", StateCategory: 0, BeginTime: types.Timestamp(time.Now())})
		err = flow.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_1"))
		Expect(err).To(Equal(bizerror.ErrWorkflowIsReferenced))
	})

	t.Run("should be able to delete workflow if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)
		s := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")

		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, s)
		Expect(err).To(BeNil())

		flowCount := 0
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.Workflow{}).Where(&domain.Workflow{ID: workflow.ID}).Count(&flowCount).Error).To(BeNil())
		Expect(flowCount).To(Equal(1))

		flowStateCount := 0
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowState{}).Where(&domain.WorkflowState{WorkflowID: workflow.ID}).Count(&flowStateCount).Error).To(BeNil())
		Expect(flowStateCount).To(Equal(3))

		flowStateTransitionCount := 0
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowStateTransition{}).
			Where(&domain.WorkflowStateTransition{WorkflowID: workflow.ID}).Count(&flowStateTransitionCount).Error).To(BeNil())
		Expect(flowStateTransitionCount).To(Equal(5))

		_, err = flow.CreatePropertyDefinition(workflow.ID,
			domain.PropertyDefinition{Name: "testProperty1", Type: "text", Title: "Test Property1"},
			s)
		Expect(err).To(BeNil())
		propertyCount := 0
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&flow.WorkflowPropertyDefinition{}).
			Where(&flow.WorkflowPropertyDefinition{WorkflowID: workflow.ID}).Count(&propertyCount).Error).To(BeNil())
		Expect(propertyCount).To(Equal(1))

		// do delete
		err = flow.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(123, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())

		// validate: related record have been deleted
		flowCount = 999
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.Workflow{}).Where(&domain.Workflow{ID: workflow.ID}).Count(&flowCount).Error).To(BeNil())
		Expect(flowCount).To(Equal(0))

		flowStateCount = 999
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowState{}).Where(&domain.WorkflowState{WorkflowID: workflow.ID}).Count(&flowStateCount).Error).To(BeNil())
		Expect(flowStateCount).To(Equal(0))

		flowStateTransitionCount = 999
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&domain.WorkflowStateTransition{}).
			Where(&domain.WorkflowStateTransition{WorkflowID: workflow.ID}).Count(&flowStateTransitionCount).Error).To(BeNil())
		Expect(flowStateTransitionCount).To(Equal(0))

		propertyCount = 999
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&flow.WorkflowPropertyDefinition{}).
			Where(&flow.WorkflowPropertyDefinition{WorkflowID: workflow.ID}).Count(&propertyCount).Error).To(BeNil())
		Expect(propertyCount).To(Equal(0))

	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		Expect(flow.DeleteWorkflow(workflow.ID, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		Expect(flow.DeleteWorkflow(workflow.ID, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkProcessStep{})
		Expect(flow.DeleteWorkflow(workflow.ID, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Work{})
		Expect(flow.DeleteWorkflow(workflow.ID, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.DeleteWorkflow(workflow.ID, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestQueryWorkflows(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should query all workflows successfully", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		_, err := flow.CreateWorkflow(
			&flow.WorkflowCreation{Name: "test workflow1", ProjectID: types.ID(1), ThemeColor: "blue", ThemeIcon: "foo", StateMachine: creationDemo.StateMachine},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeZero())
		_, err = flow.CreateWorkflow(
			&flow.WorkflowCreation{Name: "test workflow2", ProjectID: types.ID(2), ThemeColor: "blue", ThemeIcon: "bar", StateMachine: creationDemo.StateMachine},
			testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_2"))
		Expect(err).To(BeZero())

		workflows, err := flow.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1", domain.ProjectRoleManager+"_2"))
		Expect(err).To(BeNil())
		Expect(workflows).ToNot(BeNil())
		Expect(len(*workflows)).To(Equal(2))

		workflows, err = flow.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1))
		Expect(err).To(BeNil())
		Expect(workflows).ToNot(BeNil())
		Expect(len(*workflows)).To(Equal(0))

		workflows, err = flow.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(workflows).ToNot(BeNil())
		Expect(len(*workflows)).To(Equal(1))

		workflow1 := (*workflows)[0]
		Expect(workflow1.ID).ToNot(BeZero())
		Expect(workflow1.Name).To(Equal("test workflow1"))
		Expect(workflow1.ProjectID).To(Equal(types.ID(1)))
		Expect(workflow1.ThemeColor).To(Equal("blue"))
		Expect(workflow1.ThemeIcon).To(Equal("foo"))
		Expect(workflow1.CreateTime).ToNot(BeZero())
	})

	t.Run("should query by name and project id", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		_, err := flow.CreateWorkflow(
			&flow.WorkflowCreation{Name: "test workflow1", ProjectID: types.ID(1), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeZero())
		_, err = flow.CreateWorkflow(
			&flow.WorkflowCreation{Name: "test workflow2", ProjectID: types.ID(1), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeZero())
		_, err = flow.CreateWorkflow(
			&flow.WorkflowCreation{Name: "test workflow2", ProjectID: types.ID(2), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
			testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_2"))
		Expect(err).To(BeZero())

		workflows, err := flow.QueryWorkflows(
			&domain.WorkflowQuery{Name: "workflow2", ProjectID: types.ID(1)}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(workflows).ToNot(BeNil())
		Expect(len(*workflows)).To(Equal(1))

		workflow1 := (*workflows)[0]
		Expect(workflow1.ID).ToNot(BeZero())
		Expect(workflow1.Name).To(Equal("test workflow2"))
		Expect(workflow1.ThemeColor).To(Equal("blue"))
		Expect(workflow1.ThemeIcon).To(Equal("icon"))
		Expect(workflow1.ProjectID).To(Equal(types.ID(1)))
		Expect(workflow1.CreateTime).ToNot(BeZero())
	})
}

func TestUpdateWorkflowBase(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		wf, err := flow.UpdateWorkflowBase(404, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(123))
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
		Expect(wf).To(BeNil())
	})

	t.Run("should forbid to update workflow basic info without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())

		// case 1: without any permission
		wf, err := flow.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
		Expect(wf).To(BeNil())

		// case 1: with other permission
		wf, err = flow.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
		Expect(wf).To(BeNil())
	})

	t.Run("should be able to update workflow if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())

		creationCG := &flow.WorkflowCreation{Name: "test work CG", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflowCG, err := flow.CreateWorkflow(creationCG, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())

		wf, err := flow.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{Name: "updated work", ThemeColor: "red", ThemeIcon: "bar"},
			testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_333"))
		Expect(err).To(BeNil())
		Expect(wf).ToNot(BeNil())
		Expect(wf.Name).To(Equal("updated work"))
		Expect(wf.ThemeColor).To(Equal("red"))
		Expect(wf.ThemeIcon).To(Equal("bar"))
		Expect(wf.CreateTime).To(Equal(workflow.CreateTime))
		Expect(wf.ProjectID).To(Equal(workflow.ProjectID))
		Expect(wf.ID).To(Equal(workflow.ID))

		var workflowInDB domain.Workflow
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&domain.Workflow{ID: workflow.ID}).First(&workflowInDB).Error).To(BeNil())
		Expect(workflowInDB.Name).To(Equal("updated work"))
		Expect(workflowInDB.ThemeColor).To(Equal("red"))
		Expect(workflowInDB.ThemeIcon).To(Equal("bar"))
		Expect(workflowInDB.CreateTime).To(Equal(workflow.CreateTime))
		Expect(workflowInDB.ProjectID).To(Equal(workflow.ProjectID))
		Expect(workflowInDB.ID).To(Equal(workflow.ID))

		var workflowInDBCG domain.Workflow
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&domain.Workflow{ID: workflowCG.ID}).First(&workflowInDBCG).Error).To(BeNil())
		Expect(workflowInDBCG.Name).To(Equal("test work CG"))
		Expect(workflowInDBCG.ThemeColor).To(Equal("blue"))
		Expect(workflowInDBCG.ThemeIcon).To(Equal("foo"))
		Expect(workflowInDBCG.CreateTime).To(Equal(workflowCG.CreateTime))
		Expect(workflowInDBCG.ProjectID).To(Equal(workflowCG.ProjectID))
		Expect(workflowInDBCG.ID).To(Equal(workflowCG.ID))
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		wf, err := flow.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{Name: "updated work", ThemeColor: "red", ThemeIcon: "bar"},
			testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_333"))
		Expect(wf).To(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestCreateState(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist when creating state", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		err := flow.CreateState(404, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101, Transitions: []state.Transition{}}, sec)
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be forbidden without correct permissions when creating state", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		workflow, err := flow.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
		Expect(err).To(BeNil())

		creation := &flow.StateCreating{Name: "NEW", Category: 1, Order: 101, Transitions: []state.Transition{}}
		// case 1: without any permission
		err = flow.CreateState(workflow.ID, creation, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.CreateState(workflow.ID, creation, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should failed when state in transitions not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		workflow, err := flow.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
		Expect(err).To(BeNil())

		err = flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101,
			Transitions: []state.Transition{{Name: "test", From: "NotExist", To: domain.StatePending.Name}}}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))

		err = flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101,
			Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NotExist"}}}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))
	})

	t.Run("should success if everything is ok when creating state", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		workflow, err := flow.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
		Expect(err).To(BeNil())

		// do action
		err = flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 2, Order: 20002,
			Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NEW"}}}, sec)
		Expect(err).To(BeNil())

		err = flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW1", Category: 1, Order: 20001,
			Transitions: []state.Transition{}}, sec)
		Expect(err).To(BeNil())

		var affectedStates []domain.WorkflowState
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowState{WorkflowID: workflow.ID}).Order("`order` ASC").Find(&affectedStates).Error).To(BeNil())
		Expect(len(affectedStates)).To(Equal(5))
		Expect(affectedStates[3].Name).To(Equal("NEW1"))
		Expect(affectedStates[3].Category).To(Equal(state.InBacklog))
		Expect(affectedStates[4].Name).To(Equal("NEW"))
		Expect(affectedStates[4].Category).To(Equal(state.InProcess))

		var affectedTransitions []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
			Order("`create_time` ASC").Find(&affectedTransitions).Error).To(BeNil())
		Expect(len(affectedTransitions)).To(Equal(6))
		Expect(affectedTransitions[5].Name).To(Equal("test"))
		Expect(affectedTransitions[5].FromState).To(Equal(domain.StatePending.Name))
		Expect(affectedTransitions[5].ToState).To(Equal("NEW"))
	})

	t.Run("should be able to catch database error when creating state", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		workflow, err := flow.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		Expect(flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001,
			Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NEW"}}}, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		Expect(flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001, Transitions: []state.Transition{}}, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001, Transitions: []state.Transition{}}, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestUpdateWorkflowState(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		err := flow.UpdateWorkflowState(404, flow.WorkflowStateUpdating{}, sec)
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be forbidden without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// case 1: without any permission
		err = flow.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{}, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{}, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should failed when origin state not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		err = flow.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: "UNKNOWN"}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(gorm.ErrRecordNotFound.Error()))
	})

	t.Run("should failed when new state exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// failed when origin name and new name are not equals (state DOING is existed)
		err = flow.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: domain.StateDoing.Name}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrStateExisted.Error()))

		// success when origin name and new name are equals
		err = flow.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: domain.StatePending.Name}, sec)
		Expect(err).To(BeNil())
	})

	t.Run("should be able to update workflow state if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		handedEvents := []event.EventRecord{}
		event.InvokeHandlersFunc = func(record *event.EventRecord) []event.EventHandleResult {
			handedEvents = append(handedEvents, *record)
			return nil
		}
		persistedEvents := []event.EventRecord{}
		event.EventPersistCreateFunc = func(record *event.EventRecord, db *gorm.DB) error {
			persistedEvents = append(persistedEvents, *record)
			return nil
		}

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
			{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "reset", From: domain.StateDoing.Name, To: domain.StatePending.Name},
			{Name: "done", From: domain.StateDoing.Name, To: domain.StateDone.Name},
		})
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333), StateMachine: *sm}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// create work
		now := types.CurrentTimestamp()
		Expect(testDatabase.DS.GormDB(context.Background()).Create(domain.Work{
			ID: 1, Name: "test work", ProjectID: 100, CreateTime: types.Timestamp(now),
			FlowID: workflow.ID, OrderInState: 1, StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category,
			StateBeginTime: now}).Error).To(BeNil())

		// create work_process_steps
		Expect(testDatabase.DS.GormDB(context.Background()).Create(domain.WorkProcessStep{WorkID: 1, FlowID: workflow.ID,
			StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category, BeginTime: types.Timestamp(now),
			NextStateName: domain.StatePending.Name, NextStateCategory: domain.StatePending.Category}).Error).To(BeNil())

		// reset events history
		handedEvents = []event.EventRecord{}
		persistedEvents = []event.EventRecord{}
		// do action
		updating := flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: "QUEUED", Order: 2000}
		err = flow.UpdateWorkflowState(workflow.ID, updating, sec)
		Expect(err).To(BeNil())

		// check: workflow_states, workflow_state_transitions, works, work_state_transitions, work_process_steps
		var affectedStates []domain.WorkflowState
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowState{WorkflowID: workflow.ID, Name: updating.OriginName}).Find(&affectedStates).Error).To(BeNil())
		Expect(len(affectedStates)).To(BeZero())

		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowState{WorkflowID: workflow.ID, Name: updating.Name}).Find(&affectedStates).Error).To(BeNil())
		Expect(len(affectedStates)).To(Equal(1))
		Expect(affectedStates[0].Name).To(Equal(updating.Name))
		Expect(affectedStates[0].Order).To(Equal(updating.Order))
		Expect(affectedStates[0].Category).To(Equal(domain.StatePending.Category))
		Expect(affectedStates[0].WorkflowID).To(Equal(workflow.ID))
		Expect(affectedStates[0].CreateTime).ToNot(BeZero())

		var affectedStateTransitions []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
			Order("name ASC").Find(&affectedStateTransitions).Error).To(BeNil())
		Expect(len(affectedStateTransitions)).To(Equal(3))
		Expect(affectedStateTransitions[0].Name).To(Equal("begin"))
		Expect(affectedStateTransitions[0].FromState).To(Equal(updating.Name)) // updated
		Expect(affectedStateTransitions[0].ToState).To(Equal(domain.StateDoing.Name))
		Expect(affectedStateTransitions[1].Name).To(Equal("done"))
		Expect(affectedStateTransitions[1].FromState).To(Equal(domain.StateDoing.Name))
		Expect(affectedStateTransitions[1].ToState).To(Equal(domain.StateDone.Name))
		Expect(affectedStateTransitions[2].Name).To(Equal("reset"))
		Expect(affectedStateTransitions[2].FromState).To(Equal(domain.StateDoing.Name))
		Expect(affectedStateTransitions[2].ToState).To(Equal(updating.Name)) // updated

		var work domain.Work
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.Work{ID: 1}).First(&work).Error).To(BeNil())
		Expect(work.StateName).To(Equal(updating.Name))
		Expect(work.StateCategory).To(Equal(domain.StatePending.Category))

		var workProcessSteps []domain.WorkProcessStep
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkProcessStep{FlowID: workflow.ID}).First(&workProcessSteps).Error).To(BeNil())
		Expect(len(workProcessSteps)).To(Equal(1))
		Expect(workProcessSteps[0].StateName).To(Equal(updating.Name))
		Expect(workProcessSteps[0].StateCategory).To(Equal(domain.StatePending.Category))
		Expect(workProcessSteps[0].NextStateName).To(Equal(updating.Name))
		Expect(workProcessSteps[0].NextStateCategory).To(Equal(domain.StatePending.Category))

		Expect(len(handedEvents)).To(Equal(1))
		Expect((handedEvents)[0].Event).To(Equal(event.Event{SourceId: work.ID, SourceType: "WORK", SourceDesc: work.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name,
			UpdatedProperties: []event.UpdatedProperty{{
				PropertyName: "StateName", PropertyDesc: "StateName",
				OldValue: updating.OriginName, OldValueDesc: updating.OriginName,
				NewValue: updating.Name, NewValueDesc: updating.Name,
			}},
			EventCategory: event.EventCategoryExtensionUpdated}))
		Expect(time.Since((handedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		updating := flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: "QUEUED", Order: 2000}

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkProcessStep{})
		Expect(flow.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Work{})
		Expect(flow.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		Expect(flow.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		Expect(flow.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestUpdateStateRangeOrders(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		err := flow.UpdateStateRangeOrders(404, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec)
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be forbidden without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(333), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// case 1: without any permission
		err = flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}},
			testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}},
			testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should failed when origin state not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		err = flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
	})

	t.Run("should success if changes is empty or nil", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		err = flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{}, sec)
		Expect(err).To(BeNil())

		err = flow.UpdateStateRangeOrders(workflow.ID, nil, sec)
		Expect(err).To(BeNil())
	})

	t.Run("should be able to update state orders if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		sm := state.NewStateMachine([]state.State{
			{Name: "QUEUED", Category: state.InBacklog},
			{Name: "PENDING", Category: state.InBacklog},
			{Name: "DONE", Category: state.Done},
		}, []state.Transition{})
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333), StateMachine: *sm}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// do action
		err = flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{
			{State: "QUEUED", OldOlder: 1, NewOlder: 103},
			{State: "PENDING", OldOlder: 20, NewOlder: 102},
		}, sec)
		Expect(err).To(BeNil())

		var affectedStates []domain.WorkflowState
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowState{WorkflowID: workflow.ID}).Order("`order` ASC").Find(&affectedStates).Error).To(BeNil())
		Expect(len(affectedStates)).To(Equal(3))
		Expect(affectedStates[0].Name).To(Equal("PENDING"))
		Expect(affectedStates[0].Category).To(Equal(state.InBacklog))
		Expect(affectedStates[0].Order).To(Equal(102))
		Expect(affectedStates[1].Name).To(Equal("QUEUED"))
		Expect(affectedStates[1].Category).To(Equal(state.InBacklog))
		Expect(affectedStates[1].Order).To(Equal(103))
		Expect(affectedStates[2].Name).To(Equal("DONE"))
		Expect(affectedStates[2].Category).To(Equal(state.Done))
		Expect(affectedStates[2].Order).To(Equal(10003))
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		Expect(flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestCreateWorkflowStateTransitions(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		err := flow.CreateWorkflowStateTransitions(404, []state.Transition{}, sec)
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be forbidden without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// case 1: without any permission
		err = flow.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should be failed when from state or to state not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// case 1: from state not exist
		err = flow.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{{Name: "start", From: "NotExist", To: domain.StateDoing.Name}}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))

		// case 1: to state not exist
		err = flow.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{{Name: "start", From: domain.StateDoing.Name, To: "NotExist"}}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))
	})

	t.Run("should be able to create workflow transitions if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
			{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
		})

		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: *sm}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// do action
		err = flow.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{
			{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
		}, sec)
		Expect(err).To(BeNil())

		var transitionRecords []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
			Order("create_time ASC").Find(&transitionRecords).Error).To(BeNil())
		Expect(len(transitionRecords)).To(Equal(2))
		Expect(transitionRecords[0].Name).To(Equal("start"))
		Expect(transitionRecords[0].FromState).To(Equal(domain.StatePending.Name))
		Expect(transitionRecords[0].ToState).To(Equal(domain.StateDoing.Name))
		Expect(transitionRecords[1].Name).To(Equal("complete"))
		Expect(transitionRecords[1].FromState).To(Equal(domain.StateDoing.Name))
		Expect(transitionRecords[1].ToState).To(Equal(domain.StateDone.Name))
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		transitions := []state.Transition{
			{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
		}

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		Expect(flow.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowState{})
		Expect(flow.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}

func TestDeleteWorkflowStateTransitions(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return 404 when workflow not exist", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		err := flow.DeleteWorkflowStateTransitions(404, []state.Transition{}, sec)
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be forbidden without correct permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1")
		creation := &flow.WorkflowCreation{Name: "test work", ProjectID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		// case 1: without any permission
		err = flow.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))

		// case 1: with other permission
		err = flow.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_2", "reader_1"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should be able to delete workflow transitions if everything is ok", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
			{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "reset", From: domain.StateDoing.Name, To: domain.StatePending.Name},
		})

		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: *sm}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		var transitionRecords []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
			Order("create_time ASC").Find(&transitionRecords).Error).To(BeNil())
		Expect(len(transitionRecords)).To(Equal(2))

		// do action
		err = flow.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{
			{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
		}, sec)
		Expect(err).To(BeNil())

		var transitionRecordsAfterDeletion []domain.WorkflowStateTransition
		Expect(testDatabase.DS.GormDB(context.Background()).Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
			Order("create_time ASC").Find(&transitionRecordsAfterDeletion).Error).To(BeNil())
		Expect(len(transitionRecordsAfterDeletion)).To(Equal(1))
		Expect(transitionRecords[0].Name).To(Equal("reset"))
		Expect(transitionRecords[0].FromState).To(Equal(domain.StateDoing.Name))
		Expect(transitionRecords[0].ToState).To(Equal(domain.StatePending.Name))
	})

	t.Run("should be able to catch database error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_333")
		creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", ProjectID: types.ID(333),
			StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(creation, sec)
		Expect(err).To(BeNil())

		transitions := []state.Transition{
			{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
		}

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.WorkflowStateTransition{})
		Expect(flow.DeleteWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

		testDatabase.DS.GormDB(context.Background()).DropTable(&domain.Workflow{})
		Expect(flow.DeleteWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
			To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
	})
}
