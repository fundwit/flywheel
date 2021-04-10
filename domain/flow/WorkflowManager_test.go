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
		{Name: "done", From: "OPEN", To: "CLOSED"},
		{Name: "reopen", From: "CLOSED", To: "OPEN"},
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
			Expect(states[0].Order).To(Equal(10001))
			Expect(state.State{Name: states[0].Name, Category: states[0].Category, Order: states[0].Order}).To(Equal(workflow.StateMachine.States[0]))

			Expect(states[1].WorkflowID).To(Equal(workflow.ID))
			Expect(states[1].CreateTime).To(Equal(workflow.CreateTime))
			Expect(states[1].Order).To(Equal(10002))
			Expect(state.State{Name: states[1].Name, Category: states[1].Category, Order: states[1].Order}).To(Equal(workflow.StateMachine.States[1]))

			var transitions []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkflowStateTransition{}).Scan(&transitions).Error).To(BeNil())
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

	Describe("UpdateWorkflowBase", func() {
		It("should return 404 when workflow not exist", func() {
			wf, err := manager.UpdateWorkflowBase(404, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(123, []string{}))
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
			Expect(wf).To(BeNil())
		})

		It("should forbid to update workflow basic info without correct permissions", func() {
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			// case 1: without any permission
			wf, err := manager.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
			Expect(wf).To(BeNil())

			// case 1: with other permission
			wf, err = manager.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{}, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
			Expect(wf).To(BeNil())
		})

		It("should be able to update workflow if everything is ok", func() {
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_333"}))
			Expect(err).To(BeNil())

			creationCG := &flow.WorkflowCreation{Name: "test work CG", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflowCG, err := manager.CreateWorkflow(creationCG, testinfra.BuildSecCtx(100, []string{"owner_333"}))
			Expect(err).To(BeNil())

			wf, err := manager.UpdateWorkflowBase(workflow.ID, &flow.WorkflowBaseUpdation{Name: "updated work", ThemeColor: "red", ThemeIcon: "bar"},
				testinfra.BuildSecCtx(200, []string{"owner_333"}))
			Expect(err).To(BeNil())
			Expect(wf).ToNot(BeNil())
			Expect(wf.Name).To(Equal("updated work"))
			Expect(wf.ThemeColor).To(Equal("red"))
			Expect(wf.ThemeIcon).To(Equal("bar"))
			Expect(wf.CreateTime).To(Equal(workflow.CreateTime))
			Expect(wf.GroupID).To(Equal(workflow.GroupID))
			Expect(wf.ID).To(Equal(workflow.ID))

			var workflowInDB domain.Workflow
			Expect(testDatabase.DS.GormDB().Where(&domain.Workflow{ID: workflow.ID}).First(&workflowInDB).Error).To(BeNil())
			Expect(workflowInDB.Name).To(Equal("updated work"))
			Expect(workflowInDB.ThemeColor).To(Equal("red"))
			Expect(workflowInDB.ThemeIcon).To(Equal("bar"))
			Expect(workflowInDB.CreateTime).To(Equal(workflow.CreateTime))
			Expect(workflowInDB.GroupID).To(Equal(workflow.GroupID))
			Expect(workflowInDB.ID).To(Equal(workflow.ID))

			var workflowInDBCG domain.Workflow
			Expect(testDatabase.DS.GormDB().Where(&domain.Workflow{ID: workflowCG.ID}).First(&workflowInDBCG).Error).To(BeNil())
			Expect(workflowInDBCG.Name).To(Equal("test work CG"))
			Expect(workflowInDBCG.ThemeColor).To(Equal("blue"))
			Expect(workflowInDBCG.ThemeIcon).To(Equal("foo"))
			Expect(workflowInDBCG.CreateTime).To(Equal(workflowCG.CreateTime))
			Expect(workflowInDBCG.GroupID).To(Equal(workflowCG.GroupID))
			Expect(workflowInDBCG.ID).To(Equal(workflowCG.ID))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.DeleteWorkflow(workflow.ID, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
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

	Describe("CreateWorkflowStateTransitions", func() {
		It("should return 404 when workflow not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			err := manager.CreateWorkflowStateTransitions(404, []state.Transition{}, sec)
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be forbidden without correct permissions", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// case 1: without any permission
			err = manager.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be failed when from state or to state not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// case 1: from state not exist
			err = manager.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{{Name: "start", From: "NotExist", To: domain.StateDoing.Name}}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(common.ErrUnknownState.Error()))

			// case 1: to state not exist
			err = manager.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{{Name: "start", From: domain.StateDoing.Name, To: "NotExist"}}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(common.ErrUnknownState.Error()))
		})

		It("should be able to create workflow transitions if everything is ok", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
				{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
			})

			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: *sm}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// do action
			err = manager.CreateWorkflowStateTransitions(workflow.ID, []state.Transition{
				{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
			}, sec)
			Expect(err).To(BeNil())

			var transitionRecords []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
				Order("create_time ASC").Find(&transitionRecords).Error).To(BeNil())
			Expect(len(transitionRecords)).To(Equal(2))
			Expect(transitionRecords[0].Name).To(Equal("start"))
			Expect(transitionRecords[0].FromState).To(Equal(domain.StatePending.Name))
			Expect(transitionRecords[0].ToState).To(Equal(domain.StateDoing.Name))
			Expect(transitionRecords[1].Name).To(Equal("complete"))
			Expect(transitionRecords[1].FromState).To(Equal(domain.StateDoing.Name))
			Expect(transitionRecords[1].ToState).To(Equal(domain.StateDone.Name))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			transitions := []state.Transition{
				{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
			}

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			Expect(manager.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			Expect(manager.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.CreateWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})

	Describe("DeleteWorkflowStateTransitions", func() {
		It("should return 404 when workflow not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			err := manager.DeleteWorkflowStateTransitions(404, []state.Transition{}, sec)
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be forbidden without correct permissions", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// case 1: without any permission
			err = manager.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{}, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be able to delete workflow transitions if everything is ok", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
				{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "reset", From: domain.StateDoing.Name, To: domain.StatePending.Name},
			})

			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: *sm}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			var transitionRecords []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
				Order("create_time ASC").Find(&transitionRecords).Error).To(BeNil())
			Expect(len(transitionRecords)).To(Equal(2))

			// do action
			err = manager.DeleteWorkflowStateTransitions(workflow.ID, []state.Transition{
				{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
			}, sec)
			Expect(err).To(BeNil())

			var transitionRecordsAfterDeletion []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
				Order("create_time ASC").Find(&transitionRecordsAfterDeletion).Error).To(BeNil())
			Expect(len(transitionRecordsAfterDeletion)).To(Equal(1))
			Expect(transitionRecords[0].Name).To(Equal("reset"))
			Expect(transitionRecords[0].FromState).To(Equal(domain.StateDoing.Name))
			Expect(transitionRecords[0].ToState).To(Equal(domain.StatePending.Name))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			transitions := []state.Transition{
				{Name: "start", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "complete", From: domain.StateDoing.Name, To: domain.StateDone.Name},
			}

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			Expect(manager.DeleteWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.DeleteWorkflowStateTransitions(workflow.ID, transitions, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})

	Describe("UpdateWorkflowState", func() {
		It("should return 404 when workflow not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			err := manager.UpdateWorkflowState(404, flow.WorkflowStateUpdating{}, sec)
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be forbidden without correct permissions", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// case 1: without any permission
			err = manager.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{}, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{}, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should failed when origin state not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			err = manager.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: "UNKNOWN"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(gorm.ErrRecordNotFound.Error()))
		})

		It("should failed when new state exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// failed when origin name and new name are not equals (state DOING is existed)
			err = manager.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: domain.StateDoing.Name}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(common.ErrStateExisted.Error()))

			// success when origin name and new name are equals
			err = manager.UpdateWorkflowState(workflow.ID, flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: domain.StatePending.Name}, sec)
			Expect(err).To(BeNil())
		})

		It("should be able to update workflow state if everything is ok", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			sm := state.NewStateMachine([]state.State{domain.StatePending, domain.StateDoing, domain.StateDone}, []state.Transition{
				{Name: "begin", From: domain.StatePending.Name, To: domain.StateDoing.Name},
				{Name: "reset", From: domain.StateDoing.Name, To: domain.StatePending.Name},
				{Name: "done", From: domain.StateDoing.Name, To: domain.StateDone.Name},
			})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333), StateMachine: *sm}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// create work
			now := time.Now()
			Expect(testDatabase.DS.GormDB().Create(domain.Work{
				ID: 1, Name: "test work", GroupID: 100, CreateTime: now,
				FlowID: workflow.ID, OrderInState: 1, StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category,
				State: domain.StatePending, StateBeginTime: &now}).Error).To(BeNil())
			// create work_state_transitions
			Expect(testDatabase.DS.GormDB().Create(domain.WorkStateTransition{ID: 1, CreateTime: now, Creator: 100, WorkStateTransitionBrief: domain.WorkStateTransitionBrief{
				FlowID: workflow.ID, WorkID: 1, FromState: domain.StatePending.Name, ToState: domain.StatePending.Name,
			}}).Error).To(BeNil())
			// create work_process_steps
			Expect(testDatabase.DS.GormDB().Create(domain.WorkProcessStep{WorkID: 1, FlowID: workflow.ID,
				StateName: domain.StatePending.Name, StateCategory: domain.StatePending.Category, BeginTime: now}).Error).To(BeNil())

			// do action
			updating := flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: "QUEUED", Order: 2000}
			err = manager.UpdateWorkflowState(workflow.ID, updating, sec)
			Expect(err).To(BeNil())

			// check: workflow_states, workflow_state_transitions, works, work_state_transitions, work_process_steps
			var affectedStates []domain.WorkflowState
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowState{WorkflowID: workflow.ID, Name: updating.OriginName}).Find(&affectedStates).Error).To(BeNil())
			Expect(len(affectedStates)).To(BeZero())

			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowState{WorkflowID: workflow.ID, Name: updating.Name}).Find(&affectedStates).Error).To(BeNil())
			Expect(len(affectedStates)).To(Equal(1))
			Expect(affectedStates[0].Name).To(Equal(updating.Name))
			Expect(affectedStates[0].Order).To(Equal(updating.Order))
			Expect(affectedStates[0].Category).To(Equal(domain.StatePending.Category))
			Expect(affectedStates[0].WorkflowID).To(Equal(workflow.ID))
			Expect(affectedStates[0].CreateTime).ToNot(BeZero())

			var affectedStateTransitions []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
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
			Expect(testDatabase.DS.GormDB().Where(domain.Work{ID: 1}).First(&work).Error).To(BeNil())
			Expect(work.StateName).To(Equal(updating.Name))
			Expect(work.StateCategory).To(Equal(domain.StatePending.Category))

			var workStateTransition domain.WorkStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkStateTransition{ID: 1}).First(&workStateTransition).Error).To(BeNil())
			Expect(workStateTransition.FromState).To(Equal(updating.Name))
			Expect(workStateTransition.ToState).To(Equal(updating.Name))

			var workProcessSteps []domain.WorkProcessStep
			Expect(testDatabase.DS.GormDB().Where(domain.WorkProcessStep{FlowID: workflow.ID}).First(&workProcessSteps).Error).To(BeNil())
			Expect(len(workProcessSteps)).To(Equal(1))
			Expect(workProcessSteps[0].StateName).To(Equal(updating.Name))
			Expect(workProcessSteps[0].StateCategory).To(Equal(domain.StatePending.Category))
		})

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			updating := flow.WorkflowStateUpdating{OriginName: domain.StatePending.Name, Name: "QUEUED", Order: 2000}

			testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkStateTransition{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.UpdateWorkflowState(workflow.ID, updating, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})

	Describe("UpdateStateRangeOrders", func() {
		It("should return 404 when workflow not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			err := manager.UpdateStateRangeOrders(404, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec)
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be forbidden without correct permissions", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(333), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// case 1: without any permission
			err = manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}},
				testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}},
				testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should failed when origin state not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			err = manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
		})

		It("should success if changes is empty or nil", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			creation := &flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			err = manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{}, sec)
			Expect(err).To(BeNil())

			err = manager.UpdateStateRangeOrders(workflow.ID, nil, sec)
			Expect(err).To(BeNil())
		})

		It("should be able to update state orders if everything is ok", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			sm := state.NewStateMachine([]state.State{
				{Name: "QUEUED", Category: state.InBacklog},
				{Name: "PENDING", Category: state.InBacklog},
				{Name: "DONE", Category: state.Done},
			}, []state.Transition{})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333), StateMachine: *sm}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			// do action
			err = manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{
				{State: "QUEUED", OldOlder: 1, NewOlder: 103},
				{State: "PENDING", OldOlder: 20, NewOlder: 102},
			}, sec)
			Expect(err).To(BeNil())

			var affectedStates []domain.WorkflowState
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowState{WorkflowID: workflow.ID}).Order("`order` ASC").Find(&affectedStates).Error).To(BeNil())
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

		It("should be able to catch database error", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			creation := &flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := manager.CreateWorkflow(creation, sec)
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			Expect(manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.UpdateStateRangeOrders(workflow.ID, &[]flow.StateOrderRangeUpdating{{State: "UNKNOWN", OldOlder: 100, NewOlder: 101}}, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})

	Describe("CreateState", func() {
		It("should return 404 when workflow not exist when creating state", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			err := manager.CreateState(404, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101, Transitions: []state.Transition{}}, sec)
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be forbidden without correct permissions when creating state", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			workflow, err := manager.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
			Expect(err).To(BeNil())

			creation := &flow.StateCreating{Name: "NEW", Category: 1, Order: 101, Transitions: []state.Transition{}}
			// case 1: without any permission
			err = manager.CreateState(workflow.ID, creation, testinfra.BuildSecCtx(200, []string{}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))

			// case 1: with other permission
			err = manager.CreateState(workflow.ID, creation, testinfra.BuildSecCtx(200, []string{"owner_2", "reader_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should failed when state in transitions not exist", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			workflow, err := manager.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
			Expect(err).To(BeNil())

			err = manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101,
				Transitions: []state.Transition{{Name: "test", From: "NotExist", To: domain.StatePending.Name}}}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(common.ErrUnknownState.Error()))

			err = manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 101,
				Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NotExist"}}}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(common.ErrUnknownState.Error()))
		})

		It("should success if everything is ok when creating state", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_1"})
			workflow, err := manager.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", GroupID: types.ID(1),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
			Expect(err).To(BeNil())

			// do action
			err = manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 2, Order: 20002,
				Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NEW"}}}, sec)
			Expect(err).To(BeNil())

			err = manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW1", Category: 1, Order: 20001,
				Transitions: []state.Transition{}}, sec)
			Expect(err).To(BeNil())

			var affectedStates []domain.WorkflowState
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowState{WorkflowID: workflow.ID}).Order("`order` ASC").Find(&affectedStates).Error).To(BeNil())
			Expect(len(affectedStates)).To(Equal(5))
			Expect(affectedStates[3].Name).To(Equal("NEW1"))
			Expect(affectedStates[3].Category).To(Equal(state.InBacklog))
			Expect(affectedStates[4].Name).To(Equal("NEW"))
			Expect(affectedStates[4].Category).To(Equal(state.InProcess))

			var affectedTransitions []domain.WorkflowStateTransition
			Expect(testDatabase.DS.GormDB().Where(domain.WorkflowStateTransition{WorkflowID: workflow.ID}).
				Order("`create_time` ASC").Find(&affectedTransitions).Error).To(BeNil())
			Expect(len(affectedTransitions)).To(Equal(6))
			Expect(affectedTransitions[5].Name).To(Equal("test"))
			Expect(affectedTransitions[5].FromState).To(Equal(domain.StatePending.Name))
			Expect(affectedTransitions[5].ToState).To(Equal("NEW"))
		})

		It("should be able to catch database error when creating state", func() {
			sec := testinfra.BuildSecCtx(100, []string{"owner_333"})
			workflow, err := manager.CreateWorkflow(&flow.WorkflowCreation{Name: "test work", ThemeColor: "blue", ThemeIcon: "foo", GroupID: types.ID(333),
				StateMachine: domain.GenericWorkflowTemplate.StateMachine}, sec)
			Expect(err).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowStateTransition{})
			Expect(manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001,
				Transitions: []state.Transition{{Name: "test", From: domain.StatePending.Name, To: "NEW"}}}, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.WorkflowState{})
			Expect(manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001, Transitions: []state.Transition{}}, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflow_states' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Workflow{})
			Expect(manager.CreateState(workflow.ID, &flow.StateCreating{Name: "NEW", Category: 1, Order: 20001, Transitions: []state.Transition{}}, sec).Error()).
				To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".workflows' doesn't exist"))
		})
	})
})
