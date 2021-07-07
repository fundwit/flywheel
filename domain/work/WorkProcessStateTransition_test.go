package work_test

import (
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"fmt"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkStateTransition Manager", func() {
	var (
		testDatabase *testinfra.TestDatabase
		flowManager  flow.WorkflowManagerTraits
		workManager  work.WorkManagerTraits
		manager      work.WorkProcessManagerTraits
		project1     *domain.Project
		project2     *domain.Project
		lastEvents   []event.EventRecord
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error).To(BeNil())

		persistence.ActiveDataSourceManager = testDatabase.DS
		var err error
		project1, err = namespace.CreateProject(&domain.ProjectCreating{Name: "project 1", Identifier: "GR1"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", account.SystemAdminPermission.ID))
		Expect(err).To(BeNil())
		project2, err = namespace.CreateProject(&domain.ProjectCreating{Name: "project 2", Identifier: "GR2"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2", account.SystemAdminPermission.ID))
		Expect(err).To(BeNil())

		flowManager = flow.NewWorkflowManager(testDatabase.DS)
		workManager = work.NewWorkManager(testDatabase.DS, flowManager)
		manager = work.NewWorkProcessManager(testDatabase.DS, flowManager)

		lastEvents = []event.EventRecord{}
		event.EventPersistCreateFunc = func(record *event.EventRecord, db *gorm.DB) error {
			lastEvents = append(lastEvents, *record)
			return nil
		}
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateWorkStateTransition", func() {
		It("should failed if workflow is not exist", func() {
			err := manager.CreateWorkStateTransition(&domain.WorkProcessStepCreation{FlowID: 2}, testinfra.BuildSecCtx(123))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("record not found"))
			Expect(len(lastEvents)).To(BeZero())
		})
		It("should failed if transition is not acceptable", func() {
			sec := testinfra.BuildSecCtx(123, domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())

			err = manager.CreateWorkStateTransition(
				&domain.WorkProcessStepCreation{FlowID: workflow.ID, WorkID: 1, FromState: "DONE", ToState: "DOING"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("transition from DONE to DOING is not invalid"))
			Expect(len(lastEvents)).To(BeZero())
		})
		It("should failed if update work stateName failed", func() {
			err := testDatabase.DS.GormDB().DropTable(&domain.Work{}).Error
			Expect(err).To(BeNil())

			err = manager.CreateWorkStateTransition(
				&domain.WorkProcessStepCreation{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"},
				testinfra.BuildSecCtx(123))
			Expect(err).ToNot(BeNil())
			Expect(len(lastEvents)).To(BeZero())
		})
		It("should failed when work is not exist", func() {
			sec := testinfra.BuildSecCtx(types.ID(111), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())

			err = manager.CreateWorkStateTransition(
				&domain.WorkProcessStepCreation{FlowID: workflow.ID, WorkID: workflow.ID, FromState: "PENDING", ToState: "DOING"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("record not found"))
			Expect(len(lastEvents)).To(BeZero())
		})

		It("should failed when work is forbidden for current user", func() {
			sec := testinfra.BuildSecCtx(types.ID(3), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())
			detail := buildWork(workManager, "test work", workflow.ID, project1.ID, sec)

			workflowCreation = &flow.WorkflowCreation{Name: "test workflow", ProjectID: project2.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err = flowManager.CreateWorkflow(workflowCreation, testinfra.BuildSecCtx(types.ID(2), domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(err).To(BeNil())

			lastEvents = []event.EventRecord{}
			err = manager.CreateWorkStateTransition(
				&domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"},
				testinfra.BuildSecCtx(types.ID(1), domain.ProjectRoleManager+"_100", domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
			Expect(len(lastEvents)).To(BeZero())
		})
		It("should failed if stateName is not match fromState", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())
			detail := buildWork(workManager, "test work", workflow.ID, project1.ID, sec)

			lastEvents = []event.EventRecord{}
			err = manager.CreateWorkStateTransition(
				&domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
			Expect(len(lastEvents)).To(BeZero())
		})

		It("should failed if create transition record failed", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())
			detail := buildWork(workManager, "test work", workflow.ID, project1.ID, sec)

			Expect(testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{}).Error).To(BeNil())

			lastEvents = []event.EventRecord{}
			transition := domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
			err = manager.CreateWorkStateTransition(&transition,
				testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_333"))
			Expect(err).ToNot(BeZero())
			Expect(len(lastEvents)).To(BeZero())
		})

		It("should failed to create work state transition when work is archived", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())
			detail := buildWork(workManager, "test work", workflow.ID, project1.ID, sec)
			lastEvents = []event.EventRecord{}

			transition := domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DONE"}
			err = manager.CreateWorkStateTransition(&transition, sec)
			Expect(err).To(BeZero())
			Expect(workManager.ArchiveWorks([]types.ID{detail.ID}, sec)).To(BeNil())

			transition = domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DONE", ToState: "PENDING"}
			err = manager.CreateWorkStateTransition(&transition, sec)
			Expect(err).To(Equal(bizerror.ErrArchiveStatusInvalid))
		})

		It("should success when all conditions be satisfied", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
			workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
			workflow, err := flowManager.CreateWorkflow(workflowCreation, sec)
			Expect(err).To(BeNil())

			detail := buildWork(workManager, "test work", workflow.ID, project1.ID, sec)

			lastEvents = []event.EventRecord{}
			creation := domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}

			// assert: there should be a initial process step after work created
			var processSteps []domain.WorkProcessStep
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&processSteps).Error).To(BeNil())
			Expect(processSteps).ToNot(BeNil())
			Expect(len(processSteps)).To(Equal(1))
			Expect(processSteps[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
				StateName: creation.FromState, StateCategory: 1, BeginTime: detail.CreateTime, EndTime: types.Timestamp{}}))

			// do: create a new process step
			err = manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())

			// assert: event
			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
					PropertyName: "StateName", PropertyDesc: "StateName", OldValue: "PENDING", OldValueDesc: "PENDING", NewValue: "DOING", NewValueDesc: "DOING",
				}}}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			// assert: work.stateName is updated
			detail, err = workManager.WorkDetail(detail.ID.String(), sec)
			Expect(err).To(BeNil())
			Expect(detail.StateName).To(Equal("DOING"))
			Expect(detail.StateCategory).To(Equal(state.InProcess))
			Expect(detail.StateBeginTime.Time().UnixNano() >= detail.CreateTime.Time().UnixNano()).To(BeTrue())

			handleTimestamp := detail.StateBeginTime

			fmt.Println("time diff:", detail.StateBeginTime, handleTimestamp)
			Expect(detail.StateBeginTime).To(Equal(handleTimestamp))
			Expect(detail.ProcessBeginTime).To(Equal(handleTimestamp))
			Expect(detail.ProcessEndTime.IsZero()).To(BeTrue())

			// assert: should handle process step
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&processSteps).Error).To(BeNil())
			Expect(processSteps).ToNot(BeNil())
			Expect(len(processSteps)).To(Equal(2))
			Expect(processSteps[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				StateName: creation.FromState, StateCategory: 1, BeginTime: detail.CreateTime, EndTime: handleTimestamp,
				CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
				NextStateName: creation.ToState, NextStateCategory: state.InProcess}))
			Expect(processSteps[1]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
				StateName: creation.ToState, StateCategory: 2, BeginTime: handleTimestamp, EndTime: types.Timestamp{}}))

			// do: transit to done state
			creation = domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"}
			err = manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())
			// assert: processEndTime should be set
			detail, err = workManager.WorkDetail(detail.Identifier, sec)
			Expect(err).To(BeNil())
			Expect(detail.StateBeginTime.IsZero()).To(BeFalse())
			Expect(detail.ProcessBeginTime.IsZero()).To(BeFalse())
			Expect(detail.ProcessEndTime.IsZero()).To(BeFalse())

			// do: transit back to process state
			creation = domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DONE", ToState: "PENDING"}
			err = manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())
			// assert: processEndTime should be reset to nil
			detail, err = workManager.WorkDetail(detail.Identifier, sec)
			Expect(err).To(BeNil())
			Expect(detail.StateBeginTime.IsZero()).To(BeFalse())
			Expect(detail.ProcessBeginTime.IsZero()).To(BeFalse())
			Expect(detail.ProcessEndTime.IsZero()).To(BeTrue())
		})
	})
})

func buildWork(m work.WorkManagerTraits, workName string, flowId, gid types.ID, secCtx *session.Context) *domain.WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:             workName,
		ProjectID:        gid,
		FlowID:           flowId,
		InitialStateName: domain.StatePending.Name,
	}
	detail, err := m.CreateWork(workCreation, secCtx)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}
