package work_test

import (
	"context"
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/label"
	"flywheel/domain/namespace"
	"flywheel/domain/work"
	"flywheel/domain/work/checklist"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"strconv"
	"testing"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func workPropertiesTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) (*domain.WorkflowDetail,
	*domain.Project, *domain.Project, *[]event.EventRecord, *[]event.EventRecord) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	// migration
	Expect(db.DS.GormDB(context.Background()).AutoMigrate(&work.WorkLabelRelation{}, &label.Label{}, &domain.Project{},
		&domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
		&flow.WorkflowPropertyDefinition{}, &work.WorkPropertyValueRecord{},
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

func workPropertiesTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestAssignWorkPropertyValueAndQueryWOrkPropertyValues(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should block users who are not be able to view project of work", func(t *testing.T) {
		defer workPropertiesTestTeardown(t, testDatabase)
		workflow1, p1, p2, _, _ := workPropertiesTestSetup(t, &testDatabase)

		c1 := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		c2 := session.Session{Identity: session.Identity{ID: 20, Name: "user 20"},
			Perms: authority.Permissions{"manager_" + p2.ID.String()}}

		// case1: assert resource not found if work is not found
		r, err := work.AssignWorkPropertyValue(work.WorkPropertyAssign{WorkId: 404, Name: "prop1", Value: "xxx"}, &c1)
		Expect(r).To(BeNil())
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// prepare work
		w1 := buildWork("test work 1", workflow1.ID, p1.ID, &c1)

		// do action with forbidden user
		_, err = work.AssignWorkPropertyValue(work.WorkPropertyAssign{WorkId: w1.ID, Name: "prop1", Value: "xxx"}, &c2)
		// case2: access is forbidden
		Expect(err).To(Equal(bizerror.ErrForbidden))

		// case3: property definition not found within project scope
		_, err = work.AssignWorkPropertyValue(work.WorkPropertyAssign{WorkId: w1.ID, Name: "prop1", Value: "xxx"}, &c1)
		Expect(err).To(Equal(bizerror.ErrPropertyDefinitionNotFound))
	})

	t.Run("should be able to return error if failed to validate value", func(t *testing.T) {
		defer workPropertiesTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := workPropertiesTestSetup(t, &testDatabase)

		// prepare work
		c := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		w := buildWork("test work", workflow1.ID, p1.ID, &c)

		// prepare property
		pd1, err := flow.CreatePropertyDefinition(w.FlowID, domain.PropertyDefinition{Name: "prop1", Type: "number", Title: "Prop1"}, &c)
		Expect(err).To(BeNil())
		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		// assign value to work property
		req := work.WorkPropertyAssign{WorkId: w.ID, Name: "prop1", Value: "prop2 value"}
		r, err := work.AssignWorkPropertyValue(req, &c)
		Expect(r).To(BeNil())
		Expect(errors.Unwrap(err)).To(Equal(strconv.ErrSyntax))
	})

	t.Run("should be able to assign value for work property", func(t *testing.T) {
		defer workPropertiesTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := workPropertiesTestSetup(t, &testDatabase)

		// prepare work
		c := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		w := buildWork("test work", workflow1.ID, p1.ID, &c)

		// prepare property
		pd1, err := flow.CreatePropertyDefinition(w.FlowID, domain.PropertyDefinition{Name: "prop1", Type: "text", Title: "Prop1",
			Options: domain.PropertyOptions{"length": "255"}}, &c)
		Expect(err).To(BeNil())
		pd2, err := flow.CreatePropertyDefinition(w.FlowID, domain.PropertyDefinition{Name: "prop2", Type: "text", Title: "Prop2"}, &c)
		Expect(err).To(BeNil())

		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())
		Expect(work.IsPropertyDefinitionReferencedByWork(pd2.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		// assign value to work property
		req := work.WorkPropertyAssign{WorkId: w.ID, Name: "prop2", Value: "prop2 value"}
		r, err := work.AssignWorkPropertyValue(req, &c)
		Expect(err).To(BeNil())

		q := []work.WorkPropertyValueRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&work.WorkPropertyValueRecord{WorkId: w.ID}).Find(&q).Error).To(BeNil())
		Expect(len(q)).To(Equal(1))
		Expect(q[0]).To(Equal(*r))

		Expect(q[0]).To(Equal(work.WorkPropertyValueRecord{WorkId: w.ID, Name: "prop2", Value: "prop2 value", PropertyDefinitionId: pd2.ID, Type: "text"}))

		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())
		Expect(work.IsPropertyDefinitionReferencedByWork(pd2.ID, testDatabase.DS.GormDB(context.Background()))).
			To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))

		// assert the property definition can not be deleted
		Expect(flow.DeletePropertyDefinition(pd2.ID, &c)).To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))

		propValues, err := work.QueryWorkPropertyValues([]types.ID{w.ID, types.ID(404)}, &c)
		Expect(err).To(BeNil())
		Expect(len(propValues)).To(Equal(1))
		Expect(propValues).To(Equal(
			[]work.WorksPropertyValueDetail{
				{
					WorkId: w.ID,
					PropertyValues: []work.WorkPropertyValueDetail{
						{
							PropertyDefinitionId: pd1.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop1", Type: "text", Title: "Prop1",
								Options: domain.PropertyOptions{"length": "255"}},
						},
						{
							PropertyDefinitionId: pd2.ID, Value: "prop2 value",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop2", Type: "text", Title: "Prop2"},
						},
					},
				},
			}))

		// assign again
		req = work.WorkPropertyAssign{WorkId: w.ID, Name: "prop2", Value: "prop2 new value"}
		r, err = work.AssignWorkPropertyValue(req, &c)
		Expect(err).To(BeNil())

		Expect(testDatabase.DS.GormDB(context.Background()).Where(&work.WorkPropertyValueRecord{WorkId: w.ID}).Find(&q).Error).To(BeNil())
		Expect(len(q)).To(Equal(1))
		Expect(q[0]).To(Equal(*r))

		Expect(q[0]).To(Equal(work.WorkPropertyValueRecord{WorkId: w.ID, Name: "prop2", Value: "prop2 new value", PropertyDefinitionId: pd2.ID, Type: "text"}))

		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())
		Expect(work.IsPropertyDefinitionReferencedByWork(pd2.ID, testDatabase.DS.GormDB(context.Background()))).
			To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))

		propValues, err = work.QueryWorkPropertyValues([]types.ID{w.ID, types.ID(404)}, &c)
		Expect(err).To(BeNil())
		Expect(len(propValues)).To(Equal(1))
		Expect(propValues).To(Equal(
			[]work.WorksPropertyValueDetail{
				{
					WorkId: w.ID,
					PropertyValues: []work.WorkPropertyValueDetail{
						{
							PropertyDefinitionId: pd1.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop1", Type: "text", Title: "Prop1",
								Options: domain.PropertyOptions{"length": "255"}},
						},
						{
							PropertyDefinitionId: pd2.ID, Value: "prop2 new value",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop2", Type: "text", Title: "Prop2"},
						},
					},
				},
			}))

		// be able to clean
		req = work.WorkPropertyAssign{WorkId: w.ID, Name: "prop2", Value: ""}
		r, err = work.AssignWorkPropertyValue(req, &c)
		Expect(err).To(BeNil())

		Expect(testDatabase.DS.GormDB(context.Background()).Where(&work.WorkPropertyValueRecord{WorkId: w.ID}).Find(&q).Error).To(BeNil())
		Expect(len(q)).To(Equal(1))
		Expect(q[0]).To(Equal(*r))

		Expect(q[0]).To(Equal(work.WorkPropertyValueRecord{WorkId: w.ID, Name: "prop2", Value: "", PropertyDefinitionId: pd2.ID, Type: "text"}))

		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).To(BeNil())
		Expect(work.IsPropertyDefinitionReferencedByWork(pd2.ID, testDatabase.DS.GormDB(context.Background()))).
			To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))

		propValues, err = work.QueryWorkPropertyValues([]types.ID{w.ID, types.ID(404)}, &c)
		Expect(err).To(BeNil())
		Expect(len(propValues)).To(Equal(1))
		Expect(propValues).To(Equal(
			[]work.WorksPropertyValueDetail{
				{
					WorkId: w.ID,
					PropertyValues: []work.WorkPropertyValueDetail{
						{
							PropertyDefinitionId: pd1.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop1", Type: "text", Title: "Prop1",
								Options: domain.PropertyOptions{"length": "255"}},
						},
						{
							PropertyDefinitionId: pd2.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop2", Type: "text", Title: "Prop2"},
						},
					},
				},
			}))

		req = work.WorkPropertyAssign{WorkId: w.ID, Name: "prop1", Value: ""}
		r, err = work.AssignWorkPropertyValue(req, &c)
		Expect(err).To(BeNil())

		Expect(testDatabase.DS.GormDB(context.Background()).Where(&work.WorkPropertyValueRecord{WorkId: w.ID}).Find(&q).Error).To(BeNil())
		Expect(len(q)).To(Equal(2))
		Expect(q[0]).To(Equal(*r))

		Expect(q[1]).To(Equal(work.WorkPropertyValueRecord{WorkId: w.ID, Name: "prop2", Value: "", PropertyDefinitionId: pd2.ID, Type: "text"}))
		Expect(q[0]).To(Equal(work.WorkPropertyValueRecord{WorkId: w.ID, Name: "prop1", Value: "", PropertyDefinitionId: pd1.ID, Type: "text"}))

		Expect(work.IsPropertyDefinitionReferencedByWork(pd1.ID, testDatabase.DS.GormDB(context.Background()))).
			To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))
		Expect(work.IsPropertyDefinitionReferencedByWork(pd2.ID, testDatabase.DS.GormDB(context.Background()))).
			To(Equal(bizerror.ErrPropertyDefinitionIsReferenced))

		propValues, err = work.QueryWorkPropertyValues([]types.ID{w.ID, types.ID(404)}, &c)
		Expect(err).To(BeNil())
		Expect(len(propValues)).To(Equal(1))
		Expect(propValues).To(Equal(
			[]work.WorksPropertyValueDetail{
				{
					WorkId: w.ID,
					PropertyValues: []work.WorkPropertyValueDetail{
						{
							PropertyDefinitionId: pd1.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop1", Type: "text", Title: "Prop1",
								Options: domain.PropertyOptions{"length": "255"}},
						},
						{
							PropertyDefinitionId: pd2.ID, Value: "",
							PropertyDefinition: domain.PropertyDefinition{Name: "prop2", Type: "text", Title: "Prop2"},
						},
					},
				},
			}))

	})

	t.Run("should be able to assign select value for work property", func(t *testing.T) {
		defer workPropertiesTestTeardown(t, testDatabase)
		workflow1, p1, _, _, _ := workPropertiesTestSetup(t, &testDatabase)

		// prepare work
		c := session.Session{Identity: session.Identity{ID: 10, Name: "user 10"},
			Perms: authority.Permissions{"manager_" + p1.ID.String()}}
		w := buildWork("test work", workflow1.ID, p1.ID, &c)

		// prepare property
		_, err := flow.CreatePropertyDefinition(w.FlowID, domain.PropertyDefinition{Name: "prop1", Type: "select", Title: "Prop1",
			Options: domain.PropertyOptions{"selectEnums": []string{"A", "B"}}}, &c)
		Expect(err).To(BeNil())

		// assign value to work property
		req := work.WorkPropertyAssign{WorkId: w.ID, Name: "prop1", Value: "C"}
		r, err := work.AssignWorkPropertyValue(req, &c)
		Expect(r).To(BeNil())
		Expect(err.Error()).To(Equal(`invalid parameter: "C"`))

		req = work.WorkPropertyAssign{WorkId: w.ID, Name: "prop1", Value: "A"}
		r, err = work.AssignWorkPropertyValue(req, &c)
		Expect(err).To(BeNil())

		q := []work.WorkPropertyValueRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Where(&work.WorkPropertyValueRecord{WorkId: w.ID}).Find(&q).Error).To(BeNil())
		Expect(len(q)).To(Equal(1))
		Expect(q[0]).To(Equal(*r))
	})
}
