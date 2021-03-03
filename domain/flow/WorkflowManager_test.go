package flow_test

import (
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
	"time"
)

var creationDemo = &flow.WorkflowCreation{Name: "test workflow", GroupID: types.ID(1), ThemeColor: "blue", ThemeIcon: "some-icon", StateMachine: state.StateMachine{
	States: []state.State{{Name: "OPEN", Category: state.InProcess}, {Name: "CLOSED", Category: state.Done}},
	Transitions: []state.Transition{
		{Name: "done", From: state.State{Name: "OPEN", Category: state.InProcess}, To: state.State{Name: "CLOSED", Category: state.Done}},
		{Name: "reopen", From: state.State{Name: "CLOSED", Category: state.Done}, To: state.State{Name: "OPEN", Category: state.InProcess}},
	},
}}

var _ = Describe("WorkflowManager", func() {
	var (
		testDatabase *testinfra.TestDatabase
		manager      flow.WorkflowManagerTraits
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}, &domain.WorkStateTransition{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		manager = flow.NewWorkflowManager(testDatabase.DS)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("QueryWorkflows", func() {
		It("should query all workflows successfully", func() {
			_, err := manager.CreateWorkflow(
				&flow.WorkflowCreation{Name: "test workflow1", GroupID: types.ID(1), ThemeColor: "blue", ThemeIcon: "foo", StateMachine: creationDemo.StateMachine},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = manager.CreateWorkflow(
				&flow.WorkflowCreation{Name: "test workflow2", GroupID: types.ID(2), ThemeColor: "blue", ThemeIcon: "bar", StateMachine: creationDemo.StateMachine},
				testinfra.BuildSecCtx(2, []string{"owner_2"}))
			Expect(err).To(BeZero())

			workflows, err := manager.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1", "owner_2"}))
			Expect(err).To(BeNil())
			Expect(workflows).ToNot(BeNil())
			Expect(len(*workflows)).To(Equal(2))

			workflows, err = manager.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1, []string{}))
			Expect(err).To(BeNil())
			Expect(workflows).ToNot(BeNil())
			Expect(len(*workflows)).To(Equal(0))

			workflows, err = manager.QueryWorkflows(&domain.WorkflowQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(workflows).ToNot(BeNil())
			Expect(len(*workflows)).To(Equal(1))

			workflow1 := (*workflows)[0]
			Expect(workflow1.ID).ToNot(BeZero())
			Expect(workflow1.Name).To(Equal("test workflow1"))
			Expect(workflow1.GroupID).To(Equal(types.ID(1)))
			Expect(workflow1.ThemeColor).To(Equal("blue"))
			Expect(workflow1.ThemeIcon).To(Equal("foo"))
			Expect(workflow1.CreateTime).ToNot(BeZero())
		})
		It("should query by name and group id", func() {
			_, err := manager.CreateWorkflow(
				&flow.WorkflowCreation{Name: "test workflow1", GroupID: types.ID(1), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = manager.CreateWorkflow(
				&flow.WorkflowCreation{Name: "test workflow2", GroupID: types.ID(1), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = manager.CreateWorkflow(
				&flow.WorkflowCreation{Name: "test workflow2", GroupID: types.ID(2), ThemeColor: "blue", ThemeIcon: "icon", StateMachine: creationDemo.StateMachine},
				testinfra.BuildSecCtx(2, []string{"owner_2"}))
			Expect(err).To(BeZero())

			workflows, err := manager.QueryWorkflows(
				&domain.WorkflowQuery{Name: "workflow2", GroupID: types.ID(1)}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(workflows).ToNot(BeNil())
			Expect(len(*workflows)).To(Equal(1))

			workflow1 := (*workflows)[0]
			Expect(workflow1.ID).ToNot(BeZero())
			Expect(workflow1.Name).To(Equal("test workflow2"))
			Expect(workflow1.ThemeColor).To(Equal("blue"))
			Expect(workflow1.ThemeIcon).To(Equal("icon"))
			Expect(workflow1.GroupID).To(Equal(types.ID(1)))
			Expect(workflow1.CreateTime).ToNot(BeZero())
		})
	})

	Describe("CreateWorkflow", func() {
		It("should forbid to create to other group", func() {
			creation := &flow.WorkflowCreation{Name: "test workflow", GroupID: types.ID(1), StateMachine: creationDemo.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_2"}))
			Expect(workflow).To(BeNil())
			Expect(err).To(Equal(common.ErrForbidden))
		})

		It("should catch database errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			_, err := manager.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			_, err = manager.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			_, err = manager.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})

		It("should return created workflow and all data are persisted", func() {
			workflow, err := manager.CreateWorkflow(creationDemo, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(workflow.Name).To(Equal(creationDemo.Name))
			Expect(workflow.ThemeColor).To(Equal("blue"))
			Expect(workflow.ThemeIcon).To(Equal("some-icon"))
			Expect(workflow.GroupID).To(Equal(creationDemo.GroupID))
			Expect(workflow.StateMachine).To(Equal(creationDemo.StateMachine))
			Expect(workflow.ID).ToNot(BeNil())
			Expect(workflow.CreateTime).ToNot(BeNil())
			workflow.CreateTime = workflow.CreateTime.Round(time.Millisecond)

			var flows []domain.Workflow
			Expect(testDatabase.DS.GormDB().Model(&domain.Workflow{}).Scan(&flows).Error).To(BeNil())
			Expect(len(flows)).To(Equal(1))
			Expect(flows[0]).To(Equal(workflow.Workflow))

			var states []domain.WorkflowState
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowState{}).Order("`order` ASC").Scan(&states).Error).To(BeNil())
			Expect(len(states)).To(Equal(2))
			Expect(states[0].WorkflowID).To(Equal(workflow.ID))
			Expect(states[0].CreateTime).To(Equal(workflow.CreateTime))
			Expect(states[0].Order).To(Equal(0))
			Expect(state.State{Name: states[0].Name, Category: states[0].Category}).To(Equal(workflow.StateMachine.States[0]))

			Expect(states[1].WorkflowID).To(Equal(workflow.ID))
			Expect(states[1].CreateTime).To(Equal(workflow.CreateTime))
			Expect(states[1].Order).To(Equal(1))
			Expect(state.State{Name: states[1].Name, Category: states[1].Category}).To(Equal(workflow.StateMachine.States[1]))

			var transitions []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowStateTransition{}).Scan(&transitions).Error).To(BeNil())
			Expect(len(transitions)).To(Equal(2))
			Expect(transitions[0].WorkflowID).To(Equal(workflow.ID))
			Expect(transitions[0].CreateTime).To(Equal(workflow.CreateTime))
			Expect(transitions[0].Name).To(Equal(workflow.StateMachine.Transitions[1].Name))
			Expect(transitions[0].FromState).To(Equal(workflow.StateMachine.Transitions[1].From.Name))
			Expect(transitions[0].ToState).To(Equal(workflow.StateMachine.Transitions[1].To.Name))

			Expect(transitions[1].WorkflowID).To(Equal(workflow.ID))
			Expect(transitions[1].CreateTime).To(Equal(workflow.CreateTime))
			Expect(transitions[1].Name).To(Equal(workflow.StateMachine.Transitions[0].Name))
			Expect(transitions[1].FromState).To(Equal(workflow.StateMachine.Transitions[0].From.Name))
			Expect(transitions[1].ToState).To(Equal(workflow.StateMachine.Transitions[0].To.Name))
		})
	})

	Describe("DetailWorkflow", func() {
		It("should forbid to get workflow detail with permissions", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			detail, err := manager.DetailWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{"owner_2"}))
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should return 404 when workflow not exist", func() {
			detail, err := manager.DetailWorkflow(404, testinfra.BuildSecCtx(123, []string{}))
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
			Expect(detail).To(BeNil())
		})

		It("should be able to return workflow detail if everything is ok", func() {
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_333"}))
			Expect(err).To(BeNil())

			detail, err := manager.DetailWorkflow(workflow.ID, testinfra.BuildSecCtx(123, []string{"owner_333"}))
			Expect(err).To(BeNil())
			Expect(detail.ID).ToNot(BeNil())
			Expect(detail.Name).To(Equal("test work"))
			Expect(detail.ThemeColor).To(Equal("blue"))
			Expect(detail.ThemeIcon).To(Equal("foo"))
			Expect(detail.GroupID).To(Equal(types.ID(333)))
			Expect(detail.CreateTime).ToNot(BeNil())
			Expect(detail.StateMachine.States).To(Equal(domain.GenericWorkflowTemplate.StateMachine.States))
			Expect(state.SortTransitions(detail.StateMachine.Transitions)).To(
				Equal(state.SortTransitions(domain.GenericWorkflowTemplate.StateMachine.Transitions)))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			detail, err := manager.DetailWorkflow(workflow.ID, sec)
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			detail, err = manager.DetailWorkflow(workflow.ID, sec)
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			detail, err = manager.DetailWorkflow(workflow.ID, sec)
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})

	Describe("DeleteWorkflow", func() {
		It("should return 404 when workflow not exist", func() {
			err := manager.DeleteWorkflow(404, testinfra.BuildSecCtx(123, []string{}))
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should forbid to delete workflow without correct permissions", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			// case 1: without any permission
			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should forbid to delete workflow if it still be referenced by work", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().Save(&domain.Work{ID: 1, Name: "test", GroupID: 100, CreateTime: time.Now(), FlowID: workflow.ID,
				OrderInState: 1, StateName: "PENDING", StateCategory: 0, State: domain.StatePending,
				StateBeginTime: nil, ProcessBeginTime: nil, ProcessEndTime: nil})

			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{"owner_1"}))
			Expect(err).To(Equal(common.ErrWorkflowIsReferenced))
		})
		It("should forbid to delete workflow if it still be referenced by workProcessStep", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().Save(&domain.WorkProcessStep{WorkID: 1, FlowID: workflow.ID, StateName: "PENDING", StateCategory: 0, BeginTime: time.Now()})

			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{"owner_1"}))
			Expect(err).To(Equal(common.ErrWorkflowIsReferenced))
		})
		It("should forbid to delete workflow if it still be referenced by workStateTransition", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().Save(&domain.WorkStateTransition{
				ID: 1, CreateTime: time.Now(), Creator: 1,
				WorkStateTransitionBrief: domain.WorkStateTransitionBrief{
					FlowID: workflow.ID, WorkID: 1, FromState: "PENDING", ToState: "DOING"},
			})

			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(200, []string{"owner_1"}))
			Expect(err).To(Equal(common.ErrWorkflowIsReferenced))
		})

		It("should be able to delete workflow if everything is ok", func() {
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_333"}))
			Expect(err).To(BeNil())

			flowCount := 0
			Expect(testDatabase.DS.GormDB().Model(&domain.Workflow{}).Where(&domain.Workflow{ID: workflow.ID}).Count(&flowCount).Error).To(BeNil())
			Expect(flowCount).To(Equal(1))

			flowStateCount := 0
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowState{}).Where(&domain.WorkflowState{WorkflowID: workflow.ID}).Count(&flowStateCount).Error).To(BeNil())
			Expect(flowStateCount).To(Equal(3))

			flowStateTransitionCount := 0
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowStateTransition{}).
				Where(&domain.WorkflowStateTransition{WorkflowID: workflow.ID}).Count(&flowStateTransitionCount).Error).To(BeNil())
			Expect(flowStateTransitionCount).To(Equal(5))

			// do delete
			err = manager.DeleteWorkflow(workflow.ID, testinfra.BuildSecCtx(123, []string{"owner_333"}))
			Expect(err).To(BeNil())

			// validate: record have be deleted
			flowCount = 1
			Expect(testDatabase.DS.GormDB().Model(&domain.Workflow{}).Where(&domain.Workflow{ID: workflow.ID}).Count(&flowCount).Error).To(BeNil())
			Expect(flowCount).To(Equal(0))

			flowStateCount = 1
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowState{}).Where(&domain.WorkflowState{WorkflowID: workflow.ID}).Count(&flowStateCount).Error).To(BeNil())
			Expect(flowStateCount).To(Equal(0))

			flowStateTransitionCount = 1
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowStateTransition{}).
				Where(&domain.WorkflowStateTransition{WorkflowID: workflow.ID}).Count(&flowStateTransitionCount).Error).To(BeNil())
			Expect(flowStateTransitionCount).To(Equal(0))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkStateTransition{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})
})
