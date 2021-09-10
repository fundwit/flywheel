package work_test

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
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
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

func setup(t *testing.T, testDatabase **testinfra.TestDatabase) (*domain.WorkflowDetail, *domain.WorkflowDetail,
	*domain.Project, *domain.Project, *[]event.EventRecord, *[]event.EventRecord) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	Expect(db.DS.GormDB().AutoMigrate(&domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}, &checklist.CheckItem{}).Error).To(BeNil())

	persistence.ActiveDataSourceManager = db.DS
	var err error
	project1, err := namespace.CreateProject(&domain.ProjectCreating{Name: "project 1", Identifier: "GR1"},
		testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", account.SystemAdminPermission.ID))
	Expect(err).To(BeNil())
	project2, err := namespace.CreateProject(&domain.ProjectCreating{Name: "project 2", Identifier: "GR2"},
		testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2", account.SystemAdminPermission.ID))
	Expect(err).To(BeNil())

	creation := &flow.WorkflowCreation{Name: "test workflow1", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
	flowDetail, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
	Expect(err).To(BeNil())
	flowDetail.CreateTime = flowDetail.CreateTime.Round(time.Millisecond)

	creation = &flow.WorkflowCreation{Name: "test workflow2", ProjectID: project2.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
	flowDetail2, err := flow.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String()))
	Expect(err).To(BeNil())
	flowDetail2.CreateTime = flowDetail2.CreateTime.Round(time.Millisecond)

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
	work.QueryLabelBriefsOfWorkFunc = func(workId types.ID) ([]label.LabelBrief, error) {
		return nil, nil
	}
	flow.DetailWorkflowFunc = flow.DetailWorkflow

	return flowDetail, flowDetail2, project1, project2, &persistedEvents, &handedEvents
}

func teardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestCreateWork(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to catch db errors", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		testDatabase.DS.GormDB().DropTable(&domain.Work{})
		creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
		w, err := work.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(w).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(len(*handedEvents)).To(BeZero())
	})

	t.Run("should failed when initial state is unknown", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: "UNKNOWN"}
		w, err := work.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(w).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(len(*handedEvents)).To(BeZero())
	})

	t.Run("should forbid to create to other project", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, project2, _, _ := setup(t, &testDatabase)

		creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
		work, err := work.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(work).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should create new work successfully", func(t *testing.T) {
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
		w, err := work.CreateWork(creation, sec)

		Expect(err).To(BeZero())
		Expect(w).ToNot(BeZero())
		Expect(w.ID).ToNot(BeZero())
		Expect(w.Identifier).ToNot(BeZero())
		Expect(w.Name).To(Equal(creation.Name))
		Expect(w.ProjectID).To(Equal(creation.ProjectID))
		Expect(time.Until(w.CreateTime.Time()) < time.Minute).To(BeTrue())
		Expect(w.FlowID).To(Equal(flowDetail.ID))
		Expect(w.OrderInState).To(Equal(w.CreateTime.Time().UnixNano() / 1e6))
		Expect(*w.Type).To(Equal(flowDetail.Workflow))
		Expect(w.State).To(Equal(flowDetail.StateMachine.States[0]))
		Expect(w.StateBeginTime).To(Equal(w.CreateTime))
		Expect(w.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))

		// event for create should be persisted
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: w.ID, SourceType: "WORK", SourceDesc: w.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		work.QueryLabelBriefsOfWorkFunc = func(workId types.ID) ([]label.LabelBrief, error) {
			return []label.LabelBrief{
				{ID: 100, Name: "label100", ThemeColor: "red"},
				{ID: 200, Name: "label200", ThemeColor: "green"},
			}, nil
		}
		checklist.ListCheckItemsFunc = func(workId types.ID, c *session.Context) ([]checklist.CheckItem, error) {
			return []checklist.CheckItem{{Name: "test1"}}, nil
		}

		detail, err := work.DetailWork(w.ID.String(), testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeNil())
		Expect(detail).ToNot(BeNil())
		Expect(detail.ID).To(Equal(w.ID))
		Expect(detail.Name).To(Equal(creation.Name))
		Expect(detail.Identifier).To(Equal(w.Identifier))
		Expect(detail.ProjectID).To(Equal(creation.ProjectID))
		Expect(time.Until(detail.CreateTime.Time()) < time.Minute).To(BeTrue())
		Expect(*detail.Type).To(Equal(flowDetail.Workflow))
		Expect(detail.State).To(Equal(flowDetail.StateMachine.States[0]))
		Expect(detail.FlowID).To(Equal(flowDetail.ID))
		Expect(detail.OrderInState).To(Equal(w.CreateTime.Time().UnixNano() / 1e6))
		Expect(detail.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
		Expect(w.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
		//Expect(len(work.Properties)).To(Equal(0))

		Expect(detail.Labels).To(Equal([]label.LabelBrief{
			{ID: 100, Name: "label100", ThemeColor: "red"},
			{ID: 200, Name: "label200", ThemeColor: "green"},
		}))
		Expect(detail.CheckList).To(Equal([]checklist.CheckItem{{Name: "test1"}}))

		// should create init process step
		var initProcessStep []domain.WorkProcessStep
		Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&initProcessStep).Error).To(BeNil())
		Expect(initProcessStep).ToNot(BeNil())
		Expect(len(initProcessStep)).To(Equal(1))
		fmt.Println(initProcessStep[0].BeginTime, "detail:", detail.CreateTime, detail.StateBeginTime, "work:", w.CreateTime, w.StateBeginTime)
		Expect(initProcessStep[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
			CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
			StateName: detail.StateName, StateCategory: detail.State.Category, BeginTime: detail.CreateTime}))

		detail, err = work.DetailWork(w.Identifier, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeNil())
		Expect(detail).ToNot(BeNil())
		Expect(detail.ID).To(Equal(w.ID))

		// should be visible to system permissions
		detail, err = work.DetailWork(w.ID.String(), &session.Context{
			Identity: session.Identity{ID: 10, Name: "index-robot"}, Perms: authority.Permissions{account.SystemViewPermission.ID}})
		Expect(err).To(BeNil())
		Expect(detail).ToNot(BeNil())
	})

	t.Run("should create new work with highest priority successfully", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, flowDetail2, project1, project2, persistedEvents, handedEvents := setup(t, &testDatabase)

		creation := &domain.WorkCreation{Name: "test work", ProjectID: project2.ID, FlowID: flowDetail2.ID,
			InitialStateName: domain.StatePending.Name, PriorityLevel: -2}
		sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String())
		ignoreWork1, err := work.CreateWork(creation, sec)
		Expect(err).To(BeZero())
		Expect(ignoreWork1).ToNot(BeZero())
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: ignoreWork1.ID, SourceType: "WORK", SourceDesc: ignoreWork1.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		creation = &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID,
			InitialStateName: domain.StateDoing.Name}
		sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
		ignoreWork2, err := work.CreateWork(creation, sec)
		Expect(err).To(BeZero())
		Expect(ignoreWork2).ToNot(BeZero())
		Expect(ignoreWork2.OrderInState > ignoreWork1.OrderInState).To(BeTrue())
		Expect(len(*persistedEvents)).To(Equal(2))
		Expect((*persistedEvents)[1].Event).To(Equal(event.Event{SourceId: ignoreWork2.ID, SourceType: "WORK", SourceDesc: ignoreWork2.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[1].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
		creation = &domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID,
			InitialStateName: domain.StatePending.Name}
		w, err := work.CreateWork(creation, sec)
		Expect(err).To(BeZero())
		Expect(w).ToNot(BeZero())
		detail, err := work.DetailWork(w.ID.String(), sec)
		Expect(err).To(BeNil())
		Expect(detail).ToNot(BeNil())
		Expect(w.OrderInState > ignoreWork2.OrderInState).To(BeTrue())
		Expect(len(*persistedEvents)).To(Equal(3))
		Expect((*persistedEvents)[2].Event).To(Equal(event.Event{SourceId: w.ID, SourceType: "WORK", SourceDesc: w.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[2].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		creation = &domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID,
			InitialStateName: domain.StatePending.Name, PriorityLevel: -1}
		sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
		work1, err := work.CreateWork(creation, sec)
		Expect(err).To(BeZero())
		Expect(w).ToNot(BeZero())
		Expect(len(*persistedEvents)).To(Equal(4))
		Expect((*persistedEvents)[3].Event).To(Equal(event.Event{SourceId: work1.ID, SourceType: "WORK", SourceDesc: work1.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[3].Timestamp.Time()) < time.Second).To(BeTrue())
		detail1, err := work.DetailWork(work1.ID.String(), sec)
		Expect(err).To(BeNil())
		Expect(detail1).ToNot(BeNil())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		Expect(detail1.StateName).To(Equal(domain.StatePending.Name))
		Expect(detail1.StateCategory).To(Equal(domain.StatePending.Category))
		Expect(detail1.OrderInState).To(Equal(detail.OrderInState - 1))
		Expect(detail1.OrderInState > ignoreWork2.OrderInState).To(BeTrue())
	})
}

func TestDetailWork(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should forbid to get work detail with permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, project2, _, _ := setup(t, &testDatabase)

		creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
		w, err := work.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeNil())

		detail, err := work.DetailWork(w.ID.String(), testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should return error when work not found", func(t *testing.T) {
		defer teardown(t, testDatabase)
		_, _, _, project2, _, _ := setup(t, &testDatabase)

		detail, err := work.DetailWork(types.ID(404).String(), testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(detail).To(BeNil())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal(gorm.ErrRecordNotFound.Error()))
	})

	// t.Run("should return error when workflow not found", func(t *testing.T) {
	// 	defer teardown(t)
	// 	setup(t)
	// })

	// t.Run("should return error when state is invalid", func(t *testing.T) {
	// 	defer teardown(t)
	// 	setup(t)
	// })

}

func TestArchiveWorks(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should forbid to archive without permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		err = work.ArchiveWorks([]types.ID{detail.ID}, testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_123"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should not be able to archive when work is not in a completed state", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			secCtx)
		Expect(err).To(BeZero())

		err = work.ArchiveWorks([]types.ID{detail.ID}, secCtx)
		Expect(err).ToNot(BeNil())
		Expect(err).To(Equal(bizerror.ErrStateCategoryInvalid))
	})

	t.Run("should be able to catch db errors when archive work", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		testDatabase.DS.GormDB().DropTable(&domain.Work{})
		err = work.ArchiveWorks([]types.ID{detail.ID}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
	})

	t.Run("should be able to archive work by id", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		_, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StateDone.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		var works []domain.Work
		Expect(testDatabase.DS.GormDB().Find(&works).Error).To(BeNil())
		Expect(err).To(BeNil())
		Expect(works).ToNot(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect((works)[0].ArchiveTime.IsZero()).To(BeTrue())

		// do archive work
		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		workToArchive := (works)[0]
		err = work.ArchiveWorks([]types.ID{workToArchive.ID}, sec)
		Expect(err).To(BeNil())

		works = []domain.Work{}
		Expect(testDatabase.DS.GormDB().Where("id = ?", workToArchive.ID).Find(&works).Error).To(BeNil())
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		archivedWork := (works)[0]
		Expect(archivedWork.ArchiveTime.IsZero()).ToNot(BeTrue())

		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: workToArchive.ID, SourceType: "WORK", SourceDesc: workToArchive.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
				PropertyName: "ArchiveTime", PropertyDesc: "ArchiveTime",
				OldValue: workToArchive.ArchiveTime.String(), OldValueDesc: workToArchive.ArchiveTime.String(),
				NewValue: archivedWork.ArchiveTime.String(), NewValueDesc: archivedWork.ArchiveTime.String(),
			}}}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		// do archive again
		err = work.ArchiveWorks([]types.ID{workToArchive.ID}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeNil())

		works1 := []domain.Work{}
		Expect(testDatabase.DS.GormDB().Where("id = ?", workToArchive.ID).Find(&works1).Error).To(BeNil())
		Expect(err).To(BeNil())
		Expect((works1)[0].ArchiveTime).To(Equal((works)[0].ArchiveTime))
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})
}

func TestUpdateWork(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to update work", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			sec,
		)
		Expect(err).To(BeZero())

		// event handler should be invoked for creating
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		updatedWork, err := work.UpdateWork(detail.ID,
			&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())
		Expect(updatedWork).ToNot(BeNil())
		Expect(updatedWork.ID).To(Equal(detail.ID))
		Expect(updatedWork.Name).To(Equal("test work1 new"))

		// event handler should be invoked for updating
		Expect(len(*persistedEvents)).To(Equal(2))
		Expect((*persistedEvents)[1].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
				PropertyName: "Name", PropertyDesc: "Name", OldValue: detail.Name, OldValueDesc: detail.Name, NewValue: updatedWork.Name, NewValueDesc: updatedWork.Name,
			}}}))
		Expect(time.Since((*persistedEvents)[1].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		var works []domain.Work
		Expect(testDatabase.DS.GormDB().Find(&works).Error).To(BeNil())
		Expect(works).ToNot(BeNil())
		Expect(len(works)).To(Equal(1))

		Expect((works)[0].ID).To(Equal(detail.ID))
		Expect((works)[0].Name).To(Equal("test work1 new"))
	})

	t.Run("should be able to catch error when work not found", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		_, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect(*handedEvents).To(Equal(*persistedEvents))

		updatedWork, err := work.UpdateWork(404,
			&domain.WorkUpdating{Name: "test work1 new"},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(updatedWork).To(BeNil())
		Expect(err).ToNot(BeZero())
		Expect(err.Error()).To(Equal("record not found")) // thrown when check permissions
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should forbid to update work without permission", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		detail, err := work.CreateWork(&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect(*handedEvents).To(Equal(*persistedEvents))

		updatedWork, err := work.UpdateWork(detail.ID,
			&domain.WorkUpdating{Name: "test work1 new"},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_2"))
		Expect(updatedWork).To(BeNil())
		Expect(err).ToNot(BeZero())
		Expect(err.Error()).To(Equal("forbidden"))
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should be able to catch db errors", func(t *testing.T) {
		defer teardown(t, testDatabase)
		_, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		testDatabase.DS.GormDB().DropTable(&domain.Work{})

		updatedWork, err := work.UpdateWork(12345,
			&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(updatedWork).To(BeNil())
		Expect(err).ToNot(BeZero())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		Expect(len(*persistedEvents)).To(Equal(0))
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})

	t.Run("should failed when work is archived when update work", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		now := types.CurrentTimestamp()
		Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", ProjectID: project1.ID,
			CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2,
			StateName: domain.StateDone.Name, StateCategory: domain.StateDone.Category, StateBeginTime: now}).Error).To(BeNil())
		Expect(work.ArchiveWorks([]types.ID{2}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))).To(BeNil())

		updatedWork, err := work.UpdateWork(2,
			&domain.WorkUpdating{Name: "test work1 new"},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).ToNot(BeNil())
		Expect(updatedWork).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrArchiveStatusInvalid))
	})
}

func TestDeleteWork(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to delete work by id", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		_, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())
		_, err = work.CreateWork(
			&domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		var works []domain.Work
		Expect(testDatabase.DS.GormDB().Find(&works).Error).To(BeNil())
		Expect(err).To(BeNil())
		Expect(works).ToNot(BeNil())
		Expect(len(works)).To(Equal(2))

		sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		workToDelete := (works)[0]

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}

		var checklistCleanInvokedId types.ID
		checklist.CleanWorkCheckItemsDirectlyFunc = func(workId types.ID, tx *gorm.DB) error {
			checklistCleanInvokedId = workId
			Expect(tx).ToNot(BeNil())
			return nil
		}

		// do delete work
		err = work.DeleteWork(workToDelete.ID, sec)
		Expect(err).To(BeNil())

		// assert cleanWorkCheckitemDirectlyFunc has been invoked
		Expect(checklistCleanInvokedId).To(Equal(workToDelete.ID))

		// assert event handler should be invoked for deleting
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: workToDelete.ID, SourceType: "WORK", SourceDesc: workToDelete.Identifier,
			CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryDeleted}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		works = []domain.Work{}
		Expect(testDatabase.DS.GormDB().Find(&works).Error).To(BeNil())
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))

		// assert work process steps should also be deleted
		processStep := domain.WorkProcessStep{}
		Expect(testDatabase.DS.GormDB().First(&processStep, domain.WorkProcessStep{WorkID: workToDelete.ID}).Error).To(Equal(gorm.ErrRecordNotFound))
		processStep = domain.WorkProcessStep{}
		Expect(testDatabase.DS.GormDB().First(&processStep).Error).To(BeNil())
	})

	t.Run("should forbid to delete without permissions", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		err = work.DeleteWork(detail.ID, testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_123"))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("forbidden"))
	})

	t.Run("should be able to catch db errors", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, _, _ := setup(t, &testDatabase)

		detail, err := work.CreateWork(
			&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
			testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeZero())

		Expect(testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{}).Error).To(BeNil())
		err = work.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

		testDatabase.DS.GormDB().DropTable(&domain.Work{})
		err = work.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
	})
}

func TestUpdateStateRangeOrders(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should do nothing when input is empty", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		Expect(work.UpdateStateRangeOrders(nil, nil)).To(BeNil())
		Expect(work.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{}, nil)).To(BeNil())
	})

	t.Run("should be able to handle forbidden access", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		Expect(work.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
			{ID: 1, NewOlder: 3, OldOlder: 2}}, nil)).To(Equal(errors.New("record not found")))
		Expect(work.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
			{ID: 1, NewOlder: 3, OldOlder: 2}}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_404"))).To(Equal(errors.New("record not found")))
	})

	t.Run("should update order", func(t *testing.T) {
		defer teardown(t, testDatabase)
		flowDetail, _, project1, _, persistedEvents, handedEvents := setup(t, &testDatabase)

		secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
		_, err := work.CreateWork(&domain.WorkCreation{Name: "w1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
		Expect(err).To(BeZero())
		_, err = work.CreateWork(&domain.WorkCreation{Name: "w2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
		Expect(err).To(BeZero())
		_, err = work.CreateWork(&domain.WorkCreation{Name: "w3", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
		Expect(err).To(BeZero())

		// default w1 > w2 > w3
		list := []domain.Work{}
		Expect(testDatabase.DS.GormDB().Order("order_in_state ASC").Find(&list).Error).To(BeNil())
		Expect(len(list)).To(Equal(3))
		Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
		Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

		// invalid data
		Expect(work.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{{ID: list[0].ID, NewOlder: 3, OldOlder: 2}}, secCtx)).
			To(Equal(errors.New("expected affected row is 1, but actual is 0")))

		list = []domain.Work{}
		Expect(testDatabase.DS.GormDB().Order("order_in_state ASC").Find(&list).Error).To(BeNil())
		Expect(len(list)).To(Equal(3))
		Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
		Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}
		// valid data: w3 > w2 > w1
		Expect(work.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
			{ID: list[0].ID, NewOlder: list[2].OrderInState + 2, OldOlder: list[0].OrderInState},
			{ID: list[1].ID, NewOlder: list[2].OrderInState + 1, OldOlder: list[1].OrderInState}}, secCtx)).To(BeNil())

		// event handler should be invoked for update
		Expect(len(*persistedEvents)).To(Equal(2))
		lastStateOrderUpdatedWork := list[1]
		Expect((*persistedEvents)[1].Event).To(Equal(event.Event{SourceId: lastStateOrderUpdatedWork.ID, SourceType: "WORK", SourceDesc: lastStateOrderUpdatedWork.Identifier,
			CreatorId: secCtx.Identity.ID, CreatorName: secCtx.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
				PropertyName: "OrderInState", PropertyDesc: "OrderInState",
				OldValue: strconv.FormatInt(lastStateOrderUpdatedWork.OrderInState, 10), OldValueDesc: strconv.FormatInt(lastStateOrderUpdatedWork.OrderInState, 10),
				NewValue: strconv.FormatInt(list[2].OrderInState+1, 10), NewValueDesc: strconv.FormatInt(list[2].OrderInState+1, 10),
			}}}))
		Expect(time.Since((*persistedEvents)[1].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		list = []domain.Work{}
		Expect(testDatabase.DS.GormDB().Order("order_in_state ASC").Find(&list).Error).To(BeNil())
		Expect(len(list)).To(Equal(3))
		Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w3", "w2", "w1"}))
		Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())
	})
}

func TestExtendWorks(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to extend works with original order", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		flowMap := map[types.ID]domain.Workflow{}
		flow.DetailWorkflowFunc = func(id types.ID, sec *session.Context) (*domain.WorkflowDetail, error) {
			flowMap[id] = domain.Workflow{ID: id, Name: "flow-" + id.String()}
			return &domain.WorkflowDetail{
				Workflow: flowMap[id],
				StateMachine: state.StateMachine{States: []state.State{
					{Name: "PENDING", Category: state.InBacklog},
					{Name: "DONE", Category: state.Done},
				}},
			}, nil
		}
		work.QueryLabelBriefsOfWorkFunc = func(workId types.ID) ([]label.LabelBrief, error) {
			return nil, nil
		}

		ws := []domain.Work{
			{ID: 100, FlowID: 2, StateName: "PENDING"},
			{ID: 200, FlowID: 3, StateName: "DONE"},
		}
		ds, err := work.ExtendWorks(ws, nil)
		Expect(err).To(BeNil())
		t2 := flowMap[types.ID(2)]
		t3 := flowMap[types.ID(3)]
		Expect(ds).To(Equal([]work.WorkDetail{
			{
				Work:  domain.Work{ID: 100, FlowID: 2, StateName: "PENDING", StateCategory: state.InBacklog},
				Type:  &t2,
				State: state.State{Name: "PENDING", Category: state.InBacklog},
			},
			{
				Work:  domain.Work{ID: 200, FlowID: 3, StateName: "DONE", StateCategory: state.Done},
				Type:  &t3,
				State: state.State{Name: "DONE", Category: state.Done},
			},
		}))
	})
}

func TestLoadWorks(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should popup inner gorm error", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		testDatabase.DS.GormDB().DropTable(&domain.Work{})
		w, err := work.LoadWorks(1, 10)
		Expect(w).To(BeNil())
		Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
	})

	t.Run("should load works by page and order", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		db := persistence.ActiveDataSourceManager.GormDB()
		works := []domain.Work{
			{ID: 1, Identifier: "work1", Name: "work1", ProjectID: 1, FlowID: 1},
			{ID: 4, Identifier: "work4", Name: "work4", ProjectID: 2, FlowID: 1},
			{ID: 5, Identifier: "work5", Name: "work5", ProjectID: 3, FlowID: 1},
			{ID: 3, Identifier: "work3", Name: "work3", ProjectID: 4, FlowID: 1},
			{ID: 2, Identifier: "work2", Name: "work2", ProjectID: 5, FlowID: 1},
		}
		for _, w := range works {
			Expect(db.Save(&w).Error).To(BeNil())
		}

		ws, err := work.LoadWorks(2, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{works[3], works[1]}))

		ws, err = work.LoadWorks(-1, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{works[0], works[4]}))
		ws, err = work.LoadWorks(0, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{works[0], works[4]}))
		ws, err = work.LoadWorks(1, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{works[0], works[4]}))

		ws, err = work.LoadWorks(3, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{works[2]}))

		ws, err = work.LoadWorks(4, 2)
		Expect(err).To(BeNil())
		Expect(ws).To(Equal([]domain.Work{}))
	})
}
