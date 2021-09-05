package label_test

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain/label"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func setup(t *testing.T, testDatabase **testinfra.TestDatabase) {
	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	Expect(db.DS.GormDB().AutoMigrate(&label.Label{}).Error).To(BeNil())

	persistence.ActiveDataSourceManager = db.DS
}

func teardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestQueryLabels(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("only project member has permission to query labels", func(t *testing.T) {
		i := label.LabelQuery{ProjectID: 100}
		l, err := label.QueryLabels(i, &session.Context{Perms: authority.Permissions{
			account.SystemAdminPermission.ID, "admin_101"}})
		Expect(l).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))

		l, err = label.QueryLabels(i, &session.Context{Identity: session.Identity{ID: 10, Name: "user 10"}})
		Expect(l).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able query labels successfully", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100", "member_200", "admin_300"},
			Identity: session.Identity{ID: 10, Name: "user 10"}}
		l1, err := label.CreateLabel(label.LabelCreation{ProjectID: 100, Name: "test label 1"}, c)
		Expect(err).To(BeNil())
		l2, err := label.CreateLabel(label.LabelCreation{ProjectID: 100, Name: "test label 2"}, c)
		Expect(err).To(BeNil())
		_, err = label.CreateLabel(label.LabelCreation{ProjectID: 200, Name: "test label 3"}, c)
		Expect(err).To(BeNil())

		r, err := label.QueryLabels(label.LabelQuery{ProjectID: 100}, c)
		Expect(err).To(BeNil())
		Expect(len(r)).To(Equal(2))
		Expect(r[0]).To(Equal(*l1))
		Expect(r[1]).To(Equal(*l2))

		r, err = label.QueryLabels(label.LabelQuery{ProjectID: 300}, c)
		Expect(err).To(BeNil())
		Expect(len(r)).To(BeZero())
	})
}

func TestCreateLabel(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("only project member has permission to create label", func(t *testing.T) {
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		l, err := label.CreateLabel(i, &session.Context{Perms: authority.Permissions{
			account.SystemAdminPermission.ID, "admin_101"}})
		Expect(l).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))

		l, err = label.CreateLabel(i, &session.Context{Identity: session.Identity{ID: 10, Name: "user 10"}})
		Expect(l).To(BeNil())
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able create label successfully", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100"}, Identity: session.Identity{ID: 10, Name: "user 10"}}
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		l, err := label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		r := label.Label{}
		Expect(persistence.ActiveDataSourceManager.GormDB().Where("id = ?", l.ID).First(&r).Error).To(BeNil())
		Expect(r).To(Equal(*l))

		Expect(time.Since(l.CreateTime.Time()) < time.Second).To(BeTrue())
		Expect(l.ID > 0).To(BeTrue())
		l.ID = 0
		l.CreateTime = types.Timestamp{}
		Expect(*l).To(Equal(label.Label{Name: i.Name, ProjectID: i.ProjectID, CreatorID: c.Identity.ID}))
	})

	t.Run("label name should be unique in each project", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100", "admin_200"}, Identity: session.Identity{ID: 10, Name: "user 10"}}
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		_, err := label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		i = label.LabelCreation{ProjectID: 200, Name: "test label"}
		_, err = label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		_, err = label.CreateLabel(i, c)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Error 1062: Duplicate entry 'test label-200' for key 'labels.uni_name_project'"))
	})
}

func TestDeleteLabel(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("only project member has permission to delete label", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100"}, Identity: session.Identity{ID: 10, Name: "user 10"}}
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		l, err := label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		err = label.DeleteLabel(l.ID, &session.Context{Perms: authority.Permissions{
			account.SystemAdminPermission.ID, "admin_101"}})
		Expect(err).To(Equal(bizerror.ErrForbidden))

		err = label.DeleteLabel(l.ID, &session.Context{Identity: session.Identity{ID: 10, Name: "user 10"}})
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should be able delete label successfully", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100"}, Identity: session.Identity{ID: 10, Name: "user 10"}}
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		l, err := label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		var labelToDeleted label.Label
		var actualDB *gorm.DB
		label.LabelDeleteCheckFuncs = append(label.LabelDeleteCheckFuncs, func(l label.Label, tx *gorm.DB) error {
			labelToDeleted = l
			actualDB = tx
			return nil
		})
		err = label.DeleteLabel(l.ID, c)
		Expect(err).To(BeNil())
		Expect(actualDB).ToNot(BeNil())
		Expect(labelToDeleted).To(Equal(*l))

		r := label.Label{}
		Expect(persistence.ActiveDataSourceManager.GormDB().
			Where("id = ?", l.ID).First(&r).Error).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("delete action can be blocked by delete check hooks", func(t *testing.T) {
		defer teardown(t, testDatabase)
		setup(t, &testDatabase)

		c := &session.Context{Perms: authority.Permissions{"admin_100"}, Identity: session.Identity{ID: 10, Name: "user 10"}}
		i := label.LabelCreation{ProjectID: 100, Name: "test label"}
		l, err := label.CreateLabel(i, c)
		Expect(err).To(BeNil())

		labelReferencedByWork := errors.New("label referenced by work")
		label.LabelDeleteCheckFuncs = append(label.LabelDeleteCheckFuncs, func(l label.Label, tx *gorm.DB) error {
			return labelReferencedByWork
		})
		err = label.DeleteLabel(l.ID, c)
		Expect(err).To(Equal(labelReferencedByWork))

		r := label.Label{}
		Expect(persistence.ActiveDataSourceManager.GormDB().Where("id = ?", l.ID).First(&r).Error).To(BeNil())
		Expect(r).To(Equal(*l))
	})
}
