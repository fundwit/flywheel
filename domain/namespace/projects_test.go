package namespace_test

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/namespace"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Projects", func() {
	var (
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.Group{}, &domain.GroupMember{}).Error).To(BeNil())
		persistence.ActiveDataSourceManager = testDatabase.DS
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateGroup", func() {
		It("should be able to create group with a default owner member", func() {
			sec := testinfra.BuildSecCtx(types.ID(1), []string{security.SystemAdminPermission.ID})
			g, err := namespace.CreateGroup(&domain.GroupCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(err).To(BeNil())
			Expect(g).ToNot(BeNil())
			Expect(g.Name).To(Equal("demo"))
			Expect(g.Identifier).To(Equal("DEM"))
			Expect(g.NextWorkId).To(Equal(1))
			Expect(g.ID).ToNot(BeNil())
			Expect(g.CreateTime).ToNot(BeNil())
			Expect(g.Creator).To(Equal(types.ID(1)))

			var r []domain.GroupMember
			Expect(testDatabase.DS.GormDB().Find(&r).Error).To(BeNil())
			Expect(r).ToNot(BeNil())
			Expect(len(r)).To(Equal(1))
			Expect(r[0].GroupID).To(Equal(g.ID))
			Expect(r[0].MemberId).To(Equal(types.ID(1)))
			Expect(r[0].Role).To(Equal("owner"))
			Expect(r[0].CreateTime).ToNot(BeNil())

			var q []domain.Group
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(1))
			Expect(q[0].Name).To(Equal(g.Name))
			Expect(q[0].Identifier).To(Equal("DEM"))
			Expect(q[0].NextWorkId).To(Equal(1))
			Expect(q[0].ID).To(Equal(g.ID))
			Expect(q[0].CreateTime.Unix()-g.CreateTime.Unix() < 1000).To(BeTrue())
			Expect(q[0].Creator).To(Equal(g.Creator))
		})

		It("only administrator can create group", func() {
			sec := testinfra.BuildSecCtx(types.ID(1), []string{})
			g, err := namespace.CreateGroup(&domain.GroupCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(g).To(BeNil())
			Expect(err).To(Equal(bizerror.ErrForbidden))
		})

		It("should return error when database action failed", func() {
			testDatabase.DS.GormDB().DropTable(&domain.GroupMember{})

			sec := testinfra.BuildSecCtx(types.ID(1), []string{security.SystemAdminPermission.ID})
			g, err := namespace.CreateGroup(&domain.GroupCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(g).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.Group{})
			g, err = namespace.CreateGroup(&domain.GroupCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(err).ToNot(BeNil())
			Expect(g).To(BeNil())
		})
	})

	Describe("QueryGroups", func() {
		It("only administrator can query all groups", func() {
			t := time.Date(2021, 1, 1, 0, 0, 0, 0, time.Local)
			testDatabase.DS.GormDB().Save(&domain.Group{ID: 123, Identifier: "TED", Name: "test", NextWorkId: 10, CreateTime: t, Creator: 1})

			b, err := namespace.QueryProjects(testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(BeNil())
			Expect(err).To(Equal(bizerror.ErrForbidden))

			b, err = namespace.QueryProjects(testinfra.BuildSecCtx(types.ID(2), []string{security.SystemAdminPermission.ID}))
			Expect(err).To(BeNil())
			Expect(*b).To(Equal([]domain.Group{{ID: 123, Identifier: "TED", Name: "test", NextWorkId: 10, CreateTime: t, Creator: 1}}))
		})

		It("should be able to return database error", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Group{})

			b, err := namespace.QueryProjects(testinfra.BuildSecCtx(types.ID(2), []string{security.SystemAdminPermission.ID}))
			Expect(b).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".groups' doesn't exist"))
		})
	})

	Describe("UpdateGroup", func() {
		It("only administrator can update group", func() {
			t := time.Date(2021, 1, 1, 0, 0, 0, 0, time.Local)
			testDatabase.DS.GormDB().Save(&domain.Group{ID: 123, Identifier: "TED", Name: "test", NextWorkId: 10, CreateTime: t, Creator: 111})

			err := namespace.UpdateGroup(123, &domain.GroupUpdating{Name: "new name"}, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(err).To(Equal(bizerror.ErrForbidden))

			err = namespace.UpdateGroup(123, &domain.GroupUpdating{Name: "new name"}, testinfra.BuildSecCtx(types.ID(2), []string{security.SystemAdminPermission.ID}))
			Expect(err).To(BeNil())

			var q []domain.Group
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(1))
			Expect(q[0].Name).To(Equal("new name"))
			Expect(q[0].Identifier).To(Equal("TED"))
			Expect(q[0].NextWorkId).To(Equal(10))
			Expect(q[0].ID).To(Equal(types.ID(123)))
			Expect(q[0].CreateTime).To(Equal(t))
			Expect(q[0].Creator).To(Equal(types.ID(111)))
		})
	})

	Describe("QueryGroupRole", func() {
		It("should return actual role if group is accessible for user", func() {
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 2, Role: domain.RoleOwner, CreateTime: time.Now()}).Error).To(BeNil())

			b, err := namespace.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(Equal("owner"))
			Expect(err).To(BeNil())
		})
		It("should return empty if group is not accessible for user", func() {
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: time.Now()}).Error).To(BeNil())

			b, err := namespace.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(BeEmpty())
			Expect(err).To(BeNil())
		})
		It("should return empty if group member relationship is not exist", func() {
			b, err := namespace.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(BeEmpty())
			Expect(err).To(BeNil())
		})
	})

	Describe("NextWorkIdentifier", func() {
		It("should be able to generate next work identifier", func() {
			sec := testinfra.BuildSecCtx(types.ID(1), []string{security.SystemAdminPermission.ID})

			g1, err := namespace.CreateGroup(&domain.GroupCreating{Name: "group1", Identifier: "G1"}, sec)
			Expect(err).To(BeNil())
			g2, err := namespace.CreateGroup(&domain.GroupCreating{Name: "group2", Identifier: "G2"}, sec)
			Expect(err).To(BeNil())

			nextId, err := namespace.NextWorkIdentifier(g1.ID, testDatabase.DS.GormDB())
			Expect(err).To(BeNil())
			Expect(nextId).To(Equal("G1-1"))

			record := &domain.Group{}
			Expect(testDatabase.DS.GormDB().Where(&domain.Group{ID: g1.ID}).First(&record).Error).To(BeNil())
			Expect(record.NextWorkId).To(Equal(2))
			record = &domain.Group{}
			Expect(testDatabase.DS.GormDB().Where(&domain.Group{ID: g2.ID}).First(&record).Error).To(BeNil())
			Expect(record.NextWorkId).To(Equal(1))

			nextId, err = namespace.NextWorkIdentifier(g1.ID, testDatabase.DS.GormDB())
			Expect(err).To(BeNil())
			Expect(nextId).To(Equal("G1-2"))
			nextId, err = namespace.NextWorkIdentifier(g2.ID, testDatabase.DS.GormDB())
			Expect(err).To(BeNil())
			Expect(nextId).To(Equal("G2-1"))

			record = &domain.Group{}
			Expect(testDatabase.DS.GormDB().Where(&domain.Group{ID: g1.ID}).First(&record).Error).To(BeNil())
			Expect(record.NextWorkId).To(Equal(3))
			record = &domain.Group{}
			Expect(testDatabase.DS.GormDB().Where(&domain.Group{ID: g2.ID}).First(&record).Error).To(BeNil())
			Expect(record.NextWorkId).To(Equal(2))
		})
	})
})
