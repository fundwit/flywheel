package work_test

import (
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/label"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/domain/work/checklist"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"fmt"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"

	"github.com/jinzhu/gorm"
)

func workProgressTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) (*domain.WorkflowDetail,
	*domain.Project, *domain.Project, *[]event.EventRecord, *[]event.EventRecord) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	// migration
	Expect(db.DS.GormDB().AutoMigrate(&domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{},
		&work.WorkLabelRelation{}, &label.Label{}, &checklist.CheckItem{}).Error).To(BeNil())

	persistence.ActiveDataSourceManager = db.DS
	var err error
	project1, err := namespace.CreateProject(&domain.ProjectCreating{Name: "project 1", Identifier: "GR1"},
		testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", account.SystemAdminPermission.ID))
	Expect(err).To(BeNil())
	project2, err := namespace.CreateProject(&domain.ProjectCreating{Name: "project 2", Identifier: "GR2"},
		testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2", account.SystemAdminPermission.ID))
	Expect(err).To(BeNil())

	creation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
	workflowDetail, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
	Expect(err).To(BeNil())

	event.EventPersistCreateFunc = func(record *event.EventRecord, db *gorm.DB) error {
		return nil
	}

	persistedEvents := []event.EventRecord{}
	event.EventPersistCreateFunc = func(record *event.EventRecord, db *gorm.DB) error {
		persistedEvents = append(persistedEvents, *record)
		return nil
	}
	handedEvents := []event.EventRecord{}
	event.InvokeHandlersFunc = func(record *event.EventRecord) []event.EventHandleResult {
		handedEvents = append(handedEvents, *record)
		return nil
	}
	flow.DetailWorkflowFunc = flow.DetailWorkflow

	return workflowDetail, project1, project2, &persistedEvents, &handedEvents
}

func workProgressTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func buildWork(workName string, flowId, gid types.ID, secCtx *session.Context) *work.WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:             workName,
		ProjectID:        gid,
		FlowID:           flowId,
		InitialStateName: domain.StatePending.Name,
	}
	detail, err := work.CreateWork(workCreation, secCtx)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}

func TestQueryProcessSteps(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to catch db errors", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, _, _ := workProgressTestSetup(t, &testDatabase)

		secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		w, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, InitialStateName: domain.StatePending.Name}, secCtx)
		Expect(err).To(BeZero())

		testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{})
		results, err := work.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: w.ID}, secCtx)
		Expect(results).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

		testDatabase.DS.GormDB().DropTable(&domain.Work{})
		results, err = work.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: w.ID}, secCtx)
		Expect(results).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
	})

	t.Run("should return empty when work is not found", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		workProgressTestSetup(t, &testDatabase)

		work, err := work.QueryProcessSteps(
			&domain.WorkProcessStepQuery{WorkID: 1}, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1"))
		Expect(err).To(BeNil())
		Expect(len(*work)).To(Equal(0))
	})

	t.Run("should return empty when access without permissions", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, _, _ := workProgressTestSetup(t, &testDatabase)

		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		work, err := work.QueryProcessSteps(
			&domain.WorkProcessStepQuery{WorkID: detail.ID}, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2"))
		Expect(err).To(BeNil())
		Expect(len(*work)).To(Equal(0))
	})

	t.Run("should return correct result", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		workflowDetail, project1, _, _, _ := workProgressTestSetup(t, &testDatabase)

		secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		// will create init process step
		work1, err := work.CreateWork(&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, InitialStateName: domain.StatePending.Name}, secCtx)
		Expect(err).To(BeZero())

		// do transition
		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: workflowDetail.ID, WorkID: work1.ID, FromState: work1.StateName, ToState: domain.StateDoing.Name}, secCtx)
		Expect(err).To(BeNil())

		// add a record should not be query out
		now := types.CurrentTimestamp()
		Expect(testDatabase.DS.GormDB().Create(&domain.WorkProcessStep{WorkID: 3, FlowID: 2,
			StateName: "DOING", StateCategory: state.InProcess, BeginTime: now, EndTime: now}).Error).To(BeNil())

		results, err := work.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work1.ID}, secCtx)
		Expect(err).To(BeNil())
		Expect(len(*results)).To(Equal(2))
		step1 := (*results)[0]
		Expect(step1.WorkID).To(Equal(work1.ID))
		Expect(step1.FlowID).To(Equal(work1.FlowID))
		Expect(step1.StateName).To(Equal(domain.StatePending.Name))
		Expect(step1.StateCategory).To(Equal(domain.StatePending.Category))
		Expect(step1.BeginTime.Time().Round(time.Microsecond)).To(Equal(work1.CreateTime.Time().Round(time.Microsecond)))
		Expect(step1.EndTime.Time().Unix()-step1.BeginTime.Time().Unix() >= 0).To(BeTrue())
		Expect(step1.NextStateName).To(Equal("DOING"))
		Expect(step1.NextStateCategory).To(Equal(state.InProcess))
		Expect(step1.CreatorID).To(Equal(secCtx.Identity.ID))
		Expect(step1.CreatorName).To(Equal(secCtx.Identity.Nickname))

		step2 := (*results)[1]
		Expect(step2.WorkID).To(Equal(work1.ID))
		Expect(step2.FlowID).To(Equal(work1.FlowID))
		Expect(step2.StateName).To(Equal(domain.StateDoing.Name))
		Expect(step2.StateCategory).To(Equal(domain.StateDoing.Category))
		Expect(step2.BeginTime).To(Equal(step1.EndTime))
		Expect(step2.EndTime).To(Equal(types.Timestamp{}))
		Expect(step2.NextStateName).To(BeZero())
		Expect(step2.NextStateCategory).To(BeZero())
		Expect(step2.CreatorID).To(Equal(secCtx.Identity.ID))
		Expect(step2.CreatorName).To(Equal(secCtx.Identity.Nickname))

		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: workflowDetail.ID, WorkID: work1.ID, FromState: domain.StateDoing.Name, ToState: domain.StateDone.Name}, secCtx)
		Expect(err).To(BeNil())
		results, err = work.QueryProcessSteps(&domain.WorkProcessStepQuery{WorkID: work1.ID}, secCtx)
		Expect(err).To(BeNil())
		Expect(len(*results)).To(Equal(3))

		step2Finished := (*results)[1]
		Expect(step2Finished.WorkID).To(Equal(work1.ID))
		Expect(step2Finished.FlowID).To(Equal(work1.FlowID))
		Expect(step2Finished.StateName).To(Equal(domain.StateDoing.Name))
		Expect(step2Finished.StateCategory).To(Equal(domain.StateDoing.Category))
		Expect(step2Finished.BeginTime).To(Equal(step1.EndTime))
		Expect(step2Finished.EndTime.Time().Unix()-step2Finished.BeginTime.Time().Unix() >= 0).To(BeTrue())
		Expect(step2Finished.NextStateName).To(Equal(domain.StateDone.Name))
		Expect(step2Finished.NextStateCategory).To(Equal(domain.StateDone.Category))
		Expect(step2Finished.CreatorID).To(Equal(secCtx.Identity.ID))
		Expect(step2Finished.CreatorName).To(Equal(secCtx.Identity.Nickname))

		step3 := (*results)[2]
		Expect(step3.WorkID).To(Equal(work1.ID))
		Expect(step3.FlowID).To(Equal(work1.FlowID))
		Expect(step3.StateName).To(Equal(domain.StateDone.Name))
		Expect(step3.StateCategory).To(Equal(domain.StateDone.Category))
		Expect(step3.BeginTime).To(Equal(step2Finished.EndTime))
		Expect(step3.EndTime).To(BeZero())
		Expect(step3.NextStateName).To(BeZero())
		Expect(step3.NextStateCategory).To(BeZero())
		Expect(step3.CreatorID).To(Equal(secCtx.Identity.ID))
		Expect(step3.CreatorName).To(Equal(secCtx.Identity.Nickname))
	})
}

func TestCreateWorkStateTransition(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should failed if workflow is not exist", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, _, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		err := work.CreateWorkStateTransition(&domain.WorkProcessStepCreation{FlowID: 2}, testinfra.BuildSecCtx(123))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("record not found"))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed if transition is not acceptable", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(123, domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())

		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: workflow.ID, WorkID: 1, FromState: "DONE", ToState: "DOING"}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("transition from DONE to DOING is not invalid"))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed if update work stateName failed", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, _, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		err := testDatabase.DS.GormDB().DropTable(&domain.Work{}).Error
		Expect(err).To(BeNil())

		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"},
			testinfra.BuildSecCtx(123))
		Expect(err).ToNot(BeNil())
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed when work is not exist", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(111), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())

		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: workflow.ID, WorkID: workflow.ID, FromState: "PENDING", ToState: "DOING"}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("record not found"))
		Expect(len(*persistedEvents)).To(BeZero())

		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed when work is forbidden for current user", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, project2, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(3), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())
		detail := buildWork("test work", workflow.ID, project1.ID, sec)

		workflowCreation = &flow.WorkflowCreation{Name: "test workflow", ProjectID: project2.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		_, err = flow.CreateWorkflow(workflowCreation, testinfra.BuildSecCtx(types.ID(2), domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(err).To(BeNil())

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"},
			testinfra.BuildSecCtx(types.ID(1), domain.ProjectRoleManager+"_100", domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed if stateName is not match fromState", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())
		detail := buildWork("test work", workflow.ID, project1.ID, sec)

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		err = work.CreateWorkStateTransition(
			&domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"}, sec)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed if create transition record failed", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())
		detail := buildWork("test work", workflow.ID, project1.ID, sec)

		Expect(testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{}).Error).To(BeNil())

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		transition := domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
		err = work.CreateWorkStateTransition(&transition,
			testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_333"))
		Expect(err).ToNot(BeZero())
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed to create work state transition when work is archived", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())
		detail := buildWork("test work", workflow.ID, project1.ID, sec)

		transition := domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DONE"}
		err = work.CreateWorkStateTransition(&transition, sec)
		Expect(err).To(BeZero())
		Expect(work.ArchiveWorks([]types.ID{detail.ID}, sec)).To(BeNil())

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		transition = domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DONE", ToState: "PENDING"}
		err = work.CreateWorkStateTransition(&transition, sec)
		Expect(err).To(Equal(bizerror.ErrArchiveStatusInvalid))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should success when all conditions be satisfied", func(t *testing.T) {
		defer workProgressTestTeardown(t, testDatabase)
		_, project1, _, persistedEvents, handedEvents := workProgressTestSetup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(types.ID(123), domain.ProjectRoleManager+"_"+project1.ID.String())
		workflowCreation := &flow.WorkflowCreation{Name: "test workflow", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		workflow, err := flow.CreateWorkflow(workflowCreation, sec)
		Expect(err).To(BeNil())

		detail := buildWork("test work", workflow.ID, project1.ID, sec)

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
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
		err = work.CreateWorkStateTransition(&creation, sec)
		Expect(err).To(BeNil())

		// assert: event
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
				PropertyName: "StateName", PropertyDesc: "StateName", OldValue: "PENDING", OldValueDesc: "PENDING", NewValue: "DOING", NewValueDesc: "DOING",
			}}}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		// assert: work.stateName is updated
		detail, err = work.DetailWork(detail.ID.String(), sec)
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
		err = work.CreateWorkStateTransition(&creation, sec)
		Expect(err).To(BeNil())
		// assert: processEndTime should be set
		detail, err = work.DetailWork(detail.Identifier, sec)
		Expect(err).To(BeNil())
		Expect(detail.StateBeginTime.IsZero()).To(BeFalse())
		Expect(detail.ProcessBeginTime.IsZero()).To(BeFalse())
		Expect(detail.ProcessEndTime.IsZero()).To(BeFalse())

		// do: transit back to process state
		creation = domain.WorkProcessStepCreation{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DONE", ToState: "PENDING"}
		err = work.CreateWorkStateTransition(&creation, sec)
		Expect(err).To(BeNil())
		// assert: processEndTime should be reset to nil
		detail, err = work.DetailWork(detail.Identifier, sec)
		Expect(err).To(BeNil())
		Expect(detail.StateBeginTime.IsZero()).To(BeFalse())
		Expect(detail.ProcessBeginTime.IsZero()).To(BeFalse())
		Expect(detail.ProcessEndTime.IsZero()).To(BeTrue())
	})
}
