package checklist_test

import (
	"context"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/work"
	"flywheel/domain/work/checklist"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func checkitemsTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) (*domain.WorkflowDetail,
	*domain.Project, *domain.Project, *[]event.EventRecord, *[]event.EventRecord) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	// migration
	Expect(db.DS.GormDB(context.Background()).AutoMigrate(&checklist.CheckItem{}, &domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error).To(BeNil())

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

func checkitemsTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestCreateCheckitem(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// case1: assert resource not found if work is not found
		r, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: 404, Name: "item1"}, &c1)
		Expect(r).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// create check item with forbidden user
		_, err = checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item2"}, &c2)
		// case2: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able to create check item for work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, _, persistedEvents, handedEvents := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		// reset events history to exclude work create event
		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}

		ci, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())
		Expect(ci.ID).ToNot(BeZero())
		Expect(time.Since(ci.CreateTime.Time()) < time.Second).To(BeTrue())
		Expect(ci.WorkId).To(Equal(w1.ID))
		Expect(ci.Name).To(Equal("item1"))
		Expect(ci.Done).To(Equal(false))

		r := checklist.CheckItem{}
		Expect(persistence.ActiveDataSourceManager.GormDB(context.Background()).
			Where(checklist.CheckItem{WorkId: w1.ID, Name: "item1"}).First(&r).Error).
			To(BeNil())
		Expect(*ci).To(Equal(r))

		// assert event handed
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: w1.ID, SourceType: "WORK", SourceDesc: w1.Identifier,
			CreatorId: c1.Identity.ID, CreatorName: c1.Identity.Name, EventCategory: event.EventCategoryExtensionUpdated,
			UpdatedProperties: event.UpdatedProperties{
				{PropertyName: "Checklist", PropertyDesc: "Checklist", NewValue: ci.Name, NewValueDesc: ci.Name},
			},
		}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})
}

func TestListCheckitems(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// case1: assert resource not found if work is not found
		r, err := checklist.ListCheckItems(404, &c1)
		Expect(r).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// list check item with forbidden user
		_, err = checklist.ListCheckItems(w1.ID, &c2)
		// case2: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able to list check items for work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		w2 := buildWork("test work 2", workflow1.ID, p1.ID, &c1)

		cs0, err := checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs0)).To(Equal(0))

		ci1, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())
		ci2, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w2.ID, Name: "item3"}, &c1)
		Expect(err).To(BeNil())
		ci3, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item2"}, &c1)
		Expect(err).To(BeNil())

		// list
		cs1, err := checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs1)).To(Equal(2))
		Expect(cs1[0]).To(Equal(*ci1))
		Expect(cs1[1]).To(Equal(*ci3))

		cs2, err := checklist.ListCheckItems(w2.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs2)).To(Equal(1))
		Expect(cs2[0]).To(Equal(*ci2))

		// be able to list checkitems with system permissions
		cs2, err = checklist.ListCheckItems(w2.ID, &session.Session{
			Identity: session.Identity{ID: 10, Name: "index-robot"},
			Perms:    authority.Permissions{account.SystemViewPermission.ID}})
		Expect(err).To(BeNil())
		Expect(len(cs2)).To(Equal(1))
		Expect(cs2[0]).To(Equal(*ci2))
	})
}

func TestUpdateCheckitem(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		// prepare checkitem
		ci1, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())

		// delete check item with forbidden user
		err = checklist.UpdateCheckItem(ci1.ID, checklist.CheckItemUpdate{Name: "time1-update"}, &c2)
		// case1: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))

		// case2: should be failed if work is not found
		persistence.ActiveDataSourceManager.GormDB(context.Background()).Delete(&domain.Work{ID: w1.ID})
		Expect(checklist.UpdateCheckItem(ci1.ID, checklist.CheckItemUpdate{Name: "item1-update"}, &c1)).
			To(Equal(gorm.ErrRecordNotFound))

		// case2: should be failed if check item is not found
		Expect(checklist.UpdateCheckItem(404, checklist.CheckItemUpdate{Name: "item1-update"}, &c1)).
			To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be able to update check items for work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, _, persistedEvents, handedEvents := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// prepare check items
		ci1, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())
		ci3, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item2"}, &c1)
		Expect(err).To(BeNil())

		// reset events history to exclude create event
		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}

		// case1: update with nothing changed request
		Expect(checklist.UpdateCheckItem(ci1.ID, checklist.CheckItemUpdate{}, &c1)).To(BeNil())
		Expect(len(*persistedEvents)).To(BeZero())
		Expect(len(*handedEvents)).To(BeZero())

		// case2: update name and done state
		// update with name and done state
		doneState := true
		Expect(checklist.UpdateCheckItem(ci1.ID, checklist.CheckItemUpdate{Name: "updated-name", Done: &doneState}, &c1)).To(BeNil())

		// assert item changed
		cs1, err := checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs1)).To(Equal(2))
		ci1Updated := ci1
		ci1Updated.Name = "updated-name"
		ci1Updated.Done = true
		Expect(cs1[0]).To(Equal(*ci1Updated))
		Expect(cs1[1]).To(Equal(*ci3))

		// assert event handed
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: w1.ID, SourceType: "WORK", SourceDesc: w1.Identifier,
			CreatorId: c1.Identity.ID, CreatorName: c1.Identity.Name, EventCategory: event.EventCategoryExtensionUpdated,
			UpdatedProperties: event.UpdatedProperties{
				{PropertyName: "Checklist", PropertyDesc: "Checklist"},
			},
		}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		// case3: update done state to false
		doneState = false
		Expect(checklist.UpdateCheckItem(ci1.ID, checklist.CheckItemUpdate{Done: &doneState}, &c1)).To(BeNil())

		// assert item changed
		cs1, err = checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs1)).To(Equal(2))
		ci1Updated = ci1
		ci1Updated.Name = "updated-name"
		ci1Updated.Done = false
		Expect(cs1[0]).To(Equal(*ci1Updated))
		Expect(cs1[1]).To(Equal(*ci3))

		// assert new event generated
		Expect(len(*persistedEvents)).To(Equal(2))
		Expect(*handedEvents).To(Equal(*persistedEvents))
	})
}

func TestDeleteCheckitems(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		ci1, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())

		// delete check item with forbidden user
		err = checklist.DeleteCheckItem(ci1.ID, &c2)
		// case1: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))

		// case2: should be failed if work is not found
		persistence.ActiveDataSourceManager.GormDB(context.Background()).Delete(&domain.Work{ID: w1.ID})
		Expect(checklist.DeleteCheckItem(ci1.ID, &c1)).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be able to delete check items for work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, _, persistedEvents, handedEvents := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		w2 := buildWork("test work 2", workflow1.ID, p1.ID, &c1)

		ci1, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())
		ci2, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w2.ID, Name: "item3"}, &c1)
		Expect(err).To(BeNil())
		ci3, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item2"}, &c1)
		Expect(err).To(BeNil())

		// reset events history to exclude create event
		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}

		// delete
		Expect(checklist.DeleteCheckItem(ci1.ID, &c1)).To(BeNil())

		cs1, err := checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs1)).To(Equal(1))
		Expect(cs1[0]).To(Equal(*ci3))

		cs2, err := checklist.ListCheckItems(w2.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs2)).To(Equal(1))
		Expect(cs2[0]).To(Equal(*ci2))

		// assert event handed
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: w1.ID, SourceType: "WORK", SourceDesc: w1.Identifier,
			CreatorId: c1.Identity.ID, CreatorName: c1.Identity.Name, EventCategory: event.EventCategoryExtensionUpdated,
			UpdatedProperties: event.UpdatedProperties{
				{PropertyName: "Checklist", PropertyDesc: "Checklist", OldValue: ci1.Name, OldValueDesc: ci1.Name},
			},
		}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		// case2: should be successful if check item is not found
		Expect(checklist.DeleteCheckItem(ci1.ID, &c1)).To(BeNil())
		Expect(checklist.DeleteCheckItem(404, &c1)).To(BeNil())
		// assert no event handled for check items which not exist
		Expect(len(*persistedEvents)).To(Equal(1))
	})
}

func TestCleanWorkCheckitems(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// delete check item with forbidden user
		err := checklist.CleanWorkCheckItems(w1.ID, &c2)
		// case1: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))

		// case2: should be failed if work is not found
		persistence.ActiveDataSourceManager.GormDB(context.Background()).Delete(&domain.Work{ID: w1.ID})
		Expect(checklist.CleanWorkCheckItems(w1.ID, &c1)).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be able to clean check items for work", func(t *testing.T) {
		defer checkitemsTestTeardown(t, testDatabase)
		workflow1, p1, _, persistedEvents, handedEvents := checkitemsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)
		w2 := buildWork("test work 2", workflow1.ID, p1.ID, &c1)

		_, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item1"}, &c1)
		Expect(err).To(BeNil())
		ci2, err := checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w2.ID, Name: "item3"}, &c1)
		Expect(err).To(BeNil())
		_, err = checklist.CreateCheckItem(checklist.CheckItemCreation{WorkId: w1.ID, Name: "item2"}, &c1)
		Expect(err).To(BeNil())

		// reset events history to exclude create event
		*persistedEvents = []event.EventRecord{}
		*handedEvents = []event.EventRecord{}

		// clean
		Expect(checklist.CleanWorkCheckItems(w1.ID, &c1)).To(BeNil())

		cs1, err := checklist.ListCheckItems(w1.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs1)).To(Equal(0))

		cs2, err := checklist.ListCheckItems(w2.ID, &c1)
		Expect(err).To(BeNil())
		Expect(len(cs2)).To(Equal(1))
		Expect(cs2[0]).To(Equal(*ci2))

		// assert event handed
		Expect(len(*persistedEvents)).To(Equal(1))
		Expect((*persistedEvents)[0].Event).To(Equal(event.Event{SourceId: w1.ID, SourceType: "WORK", SourceDesc: w1.Identifier,
			CreatorId: c1.Identity.ID, CreatorName: c1.Identity.Name, EventCategory: event.EventCategoryExtensionUpdated,
			UpdatedProperties: event.UpdatedProperties{
				{PropertyName: "Checklist", PropertyDesc: "Checklist"},
			},
		}))
		Expect(time.Since((*persistedEvents)[0].Timestamp.Time()) < time.Second).To(BeTrue())
		Expect(*handedEvents).To(Equal(*persistedEvents))

		// case2: should be successful if work has no check items any more
		Expect(checklist.CleanWorkCheckItems(w1.ID, &c1)).To(BeNil())
	})
}

func buildWork(workName string, flowId, gid types.ID, s *session.Session) *work.WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:             workName,
		ProjectID:        gid,
		FlowID:           flowId,
		InitialStateName: domain.StatePending.Name,
	}
	detail, err := work.CreateWork(workCreation, s)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}
