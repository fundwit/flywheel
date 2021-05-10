package work_test

import (
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/persistence"
	"flywheel/testinfra"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("WorkProcessManager", func() {
	var (
		workProcessManager *work.WorkProcessManager
		workManager        *work.WorkManager
		testDatabase       *testinfra.TestDatabase
		workflowDetail     *domain.WorkflowDetail
		group1             *domain.Group
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		// migration
		Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.Group{}, &domain.GroupMember{}, &domain.Work{}, &domain.WorkProcessStep{}, &domain.WorkStateTransition{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error).To(BeNil())

		persistence.ActiveDataSourceManager = testDatabase.DS
		var err error
		group1, err = namespace.CreateGroup(&domain.GroupCreating{Name: "group 1", Identifier: "GR1"}, testinfra.BuildSecCtx(100, []string{"owner_1"}))
		Expect(err).To(BeNil())

		workflowManager := flow.NewWorkflowManager(testDatabase.DS)
		workProcessManager = work.NewWorkProcessManager(testDatabase.DS, workflowManager)
		creation := &flow.WorkflowCreation{Name: "test workflow", GroupID: group1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflowDetail, err = workflowManager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_" + group1.ID.String()}))
		Expect(err).To(BeNil())

		workManager = work.NewWorkManager(testDatabase.DS, workflowManager)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("QueryProcessSteps", func() {
		It("should be able to catch db errors", func() {
			secCtx := testinfra.BuildSecCtx(1, []string{"owner_" + group1.ID.String()})
			work, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: group1.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())

			testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{})
			results, err := workProcessManager.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work.ID}, secCtx)
			Expect(results).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			results, err = workProcessManager.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work.ID}, secCtx)
			Expect(results).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})

		It("should return empty when work is not found", func() {
			work, err := workProcessManager.QueryProcessSteps(
				&domain.WorkProcessStepQuery{WorkID: 1}, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(len(*work)).To(Equal(0))
		})

		It("should return empty when access without permissions", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: group1.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, []string{"owner_" + group1.ID.String()}))
			Expect(err).To(BeZero())

			work, err := workProcessManager.QueryProcessSteps(
				&domain.WorkProcessStepQuery{WorkID: detail.ID}, testinfra.BuildSecCtx(100, []string{"owner_2"}))
			Expect(err).To(BeNil())
			Expect(len(*work)).To(Equal(0))
		})

		It("should return correct result", func() {
			secCtx := testinfra.BuildSecCtx(1, []string{"owner_" + group1.ID.String()})
			// will create init process step
			work1, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", GroupID: group1.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())

			// do transition
			workFlowManager := flow.NewWorkflowManager(testDatabase.DS)
			workProcessManager := work.NewWorkProcessManager(testDatabase.DS, workFlowManager)
			_, err = workProcessManager.CreateWorkStateTransition(
				&domain.WorkStateTransitionBrief{FlowID: workflowDetail.ID, WorkID: work1.ID, FromState: work1.StateName, ToState: domain.StateDoing.Name}, secCtx)
			Expect(err).To(BeNil())

			// add a record should not be query out
			now := time.Now()
			Expect(testDatabase.DS.GormDB().Create(&domain.WorkProcessStep{WorkID: 3, FlowID: 2,
				StateName: "DOING", StateCategory: state.InProcess, BeginTime: now, EndTime: &now}).Error).To(BeNil())

			results, err := workProcessManager.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work1.ID}, secCtx)
			Expect(err).To(BeNil())
			Expect(len(*results)).To(Equal(2))
			step1 := (*results)[0]
			Expect(step1.WorkID).To(Equal(work1.ID))
			Expect(step1.FlowID).To(Equal(work1.FlowID))
			Expect(step1.StateName).To(Equal(domain.StatePending.Name))
			Expect(step1.StateCategory).To(Equal(domain.StatePending.Category))
			Expect(step1.BeginTime.Unix()).To(Equal(work1.CreateTime.Unix()))
			Expect(step1.EndTime.Unix()-step1.BeginTime.Unix() >= 0).To(BeTrue())

			step2 := (*results)[1]
			Expect(step2.WorkID).To(Equal(work1.ID))
			Expect(step2.FlowID).To(Equal(work1.FlowID))
			Expect(step2.StateName).To(Equal(domain.StateDoing.Name))
			Expect(step2.StateCategory).To(Equal(domain.StateDoing.Category))
			Expect(step2.BeginTime.Unix()).To(Equal(step1.EndTime.Unix()))
			Expect(step2.EndTime).To(BeNil())

			_, err = workProcessManager.CreateWorkStateTransition(
				&domain.WorkStateTransitionBrief{FlowID: workflowDetail.ID, WorkID: work1.ID, FromState: domain.StateDoing.Name, ToState: domain.StateDone.Name}, secCtx)
			Expect(err).To(BeNil())
			results, err = workProcessManager.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work1.ID}, secCtx)
			Expect(err).To(BeNil())
			Expect(len(*results)).To(Equal(2))

			step2Finished := (*results)[1]
			Expect(step2Finished.WorkID).To(Equal(work1.ID))
			Expect(step2Finished.FlowID).To(Equal(work1.FlowID))
			Expect(step2Finished.StateName).To(Equal(domain.StateDoing.Name))
			Expect(step2Finished.StateCategory).To(Equal(domain.StateDoing.Category))
			Expect(step2Finished.BeginTime.Unix()).To(Equal(step1.EndTime.Unix()))
			Expect(step2Finished.EndTime.Unix()-step2Finished.BeginTime.Unix() >= 0).To(BeTrue())
		})
	})
})
