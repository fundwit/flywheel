package work

import (
	"context"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/label"
	"flywheel/domain/namespace"
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

func workLabelsTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) (*domain.WorkflowDetail,
	*domain.Project, *domain.Project, *[]event.EventRecord, *[]event.EventRecord) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	// migration
	Expect(db.DS.GormDB(context.Background()).AutoMigrate(&WorkLabelRelation{}, &label.Label{}, &domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
		&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}, &checklist.CheckItem{}).Error).To(BeNil())

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

func workLabelsTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestCreateWorkLabelRelation(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer workLabelsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := workLabelsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// case1: assert resource not found if work is not found
		r, err := CreateWorkLabelRelation(WorkLabelRelationReq{WorkId: 404, LabelId: 404}, &c1)
		Expect(r).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// create work-label-relation with forbidden user
		_, err = CreateWorkLabelRelation(WorkLabelRelationReq{WorkId: w1.ID, LabelId: 404}, &c2)
		// case2: assert access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))

		// case3: assert label not found
		l2, err := label.CreateLabel(label.LabelCreation{ProjectID: p2.ID, Name: "test label", ThemeColor: "red"}, &c2)
		Expect(err).To(BeNil())

		_, err = CreateWorkLabelRelation(WorkLabelRelationReq{WorkId: w1.ID, LabelId: l2.ID}, &c1)
		Expect(err).To(Equal(bizerror.ErrLabelNotFound))
	})

	t.Run("should be able to create work label relation", func(t *testing.T) {
		defer workLabelsTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := workLabelsTestSetup(t, &testDatabase)

		// prepare work
		c := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		w := buildWork("test work", workflow1.ID, p1.ID, &c)

		// prepare label
		l, err := label.CreateLabel(label.LabelCreation{ProjectID: p1.ID, Name: "test label", ThemeColor: "red"}, &c)
		Expect(err).To(BeNil())

		// create work-label-relation
		req := WorkLabelRelationReq{WorkId: w.ID, LabelId: l.ID}
		r, err := CreateWorkLabelRelation(req, &c)
		Expect(err).To(BeNil())
		Expect(time.Since(r.CreateTime.Time()) < time.Second).To(BeTrue())

		q := WorkLabelRelation{}
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&WorkLabelRelation{WorkId: w.ID, LabelId: l.ID}).First(&q).Error).To(BeNil())
		Expect(q).To(Equal(*r))

		q.CreateTime = types.Timestamp{}
		Expect(q).To(Equal(WorkLabelRelation{WorkId: w.ID, LabelId: l.ID, CreatorId: c.Identity.ID}))
	})
}

func TestDeleteWorkLabelRelation(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able pop up error", func(t *testing.T) {
		defer workLabelsTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := workLabelsTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// case1: assert param errors
		Expect(DeleteWorkLabelRelation(WorkLabelRelationReq{}, &c1)).
			To(Equal(bizerror.ErrInvalidArguments))

		// case2: assert resource not found if work is not found
		Expect(DeleteWorkLabelRelation(WorkLabelRelationReq{WorkId: 404, LabelId: 404}, &c1)).
			To(Equal(gorm.ErrRecordNotFound))

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// prepare label
		l1, err := label.CreateLabel(label.LabelCreation{ProjectID: p1.ID, Name: "test label", ThemeColor: "red"}, &c1)
		Expect(err).To(BeNil())

		// prepare work-label-relation
		_, err = CreateWorkLabelRelation(WorkLabelRelationReq{WorkId: w1.ID, LabelId: l1.ID}, &c1)
		Expect(err).To(BeNil())

		// case3: assert access is forbidden
		Expect(DeleteWorkLabelRelation(WorkLabelRelationReq{WorkId: w1.ID, LabelId: l1.ID}, &c2)).
			To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able to delete work-label-relation", func(t *testing.T) {
		defer workLabelsTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := workLabelsTestSetup(t, &testDatabase)

		label.LabelDeleteCheckFuncs = append(label.LabelDeleteCheckFuncs, IsLabelReferencedByWork)

		// prepare work
		c := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		w := buildWork("test work", workflow1.ID, p1.ID, &c)

		// prepare label
		l, err := label.CreateLabel(label.LabelCreation{ProjectID: p1.ID, Name: "test label", ThemeColor: "red"}, &c)
		Expect(err).To(BeNil())
		Expect(IsLabelReferencedByWork(*l, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		// prepare work-label-relation
		req := WorkLabelRelationReq{WorkId: w.ID, LabelId: l.ID}
		_, err = CreateWorkLabelRelation(req, &c)
		Expect(err).To(BeNil())
		Expect(IsLabelReferencedByWork(*l, testDatabase.DS.GormDB(context.Background()))).To(Equal(bizerror.ErrLabelIsReferenced))
		// assert LabelDeleteCheckFuncs is registered
		Expect(label.DeleteLabel(l.ID, &c)).To(Equal(bizerror.ErrLabelIsReferenced))
		// assert query label briefs of work
		b, err := QueryLabelBriefsOfWork([]types.ID{w.ID}, &c)
		Expect(err).To(BeNil())
		Expect(b).To(Equal([]WorkLabelBrief{{WorkID: w.ID, LabelID: l.ID, LabelName: l.Name, LabelThemeColor: l.ThemeColor}}))

		// do delete work-label-relation
		Expect(DeleteWorkLabelRelation(req, &c)).To(BeNil())
		Expect(IsLabelReferencedByWork(*l, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		b, err = QueryLabelBriefsOfWork([]types.ID{w.ID}, &c)
		Expect(err).To(BeNil())
		Expect(len(b)).To(BeZero())

		// assert relation already been delete from database
		q := WorkLabelRelation{}
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&WorkLabelRelation{WorkId: w.ID, LabelId: l.ID}).
			First(&q).Error).To(Equal(gorm.ErrRecordNotFound))
	})
}

func TestClearWorkLabelRelations(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to clear relations with label for a work", func(t *testing.T) {
		defer workLabelsTestTeardown(t, testDatabase)
		workLabelsTestSetup(t, &testDatabase)

		// create work-label-relations
		Expect(testDatabase.DS.GormDB(context.Background()).Save(
			&WorkLabelRelation{WorkId: 100, LabelId: 10, CreateTime: types.CurrentTimestamp(), CreatorId: 1}).Error).
			To(BeNil())
		Expect(testDatabase.DS.GormDB(context.Background()).Save(
			&WorkLabelRelation{WorkId: 100, LabelId: 20, CreateTime: types.CurrentTimestamp(), CreatorId: 1}).Error).
			To(BeNil())
		Expect(testDatabase.DS.GormDB(context.Background()).Save(
			&WorkLabelRelation{WorkId: 110, LabelId: 10, CreateTime: types.CurrentTimestamp(), CreatorId: 1}).Error).
			To(BeNil())

		Expect(clearWorkLabelRelations(0, testDatabase.DS.GormDB(context.Background()))).To(BeNil())
		Expect(clearWorkLabelRelations(100, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		left := []WorkLabelRelation{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&WorkLabelRelation{}).Find(&left).Error).To(BeNil())
		Expect(len(left)).To(Equal(1))
		Expect(left[0].WorkId).To(Equal(types.ID(110)))
	})
}

func buildWork(workName string, flowId, gid types.ID, s *session.Session) *WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:             workName,
		ProjectID:        gid,
		FlowID:           flowId,
		InitialStateName: domain.StatePending.Name,
	}
	detail, err := CreateWork(workCreation, s)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}
