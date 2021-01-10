package namespace_test

import (
	"flywheel/domain"
	"flywheel/domain/namespace"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
	"time"
)

var _ = Describe("GroupManager", func() {
	var (
		testDatabase *testinfra.TestDatabase
		m            namespace.GroupManagerTraits
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Group{}, &domain.GroupMember{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		m = namespace.NewGroupManager(testDatabase.DS)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateGroup", func() {
		It("should be able to create group with a default owner member", func() {
			g, err := m.CreateGroup("demo", testinfra.BuildSecCtx(types.ID(1), nil))
			Expect(err).To(BeNil())
			Expect(g).ToNot(BeNil())
			Expect(g.Name).To(Equal("demo"))
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
			Expect(q[0].ID).To(Equal(g.ID))
			Expect(q[0].CreateTime.Unix()-g.CreateTime.Unix() < 1000).To(BeTrue())
			Expect(q[0].Creator).To(Equal(g.Creator))
		})

		It("should return error when database action failed", func() {
			testDatabase.DS.GormDB().DropTable(&domain.GroupMember{})

			g, err := m.CreateGroup("demo", testinfra.BuildSecCtx(types.ID(1), nil))
			Expect(err).ToNot(BeNil())
			Expect(g).To(BeNil())

			testDatabase.DS.GormDB().DropTable(&domain.Group{})
			g, err = m.CreateGroup("demo", testinfra.BuildSecCtx(types.ID(1), nil))
			Expect(err).ToNot(BeNil())
			Expect(g).To(BeNil())
		})
	})

	Describe("QueryGroupRole", func() {
		It("should return actual role if group is accessible for user", func() {
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 2, Role: domain.RoleOwner, CreateTime: time.Now()}).Error).To(BeNil())

			b, err := m.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(Equal("owner"))
			Expect(err).To(BeNil())
		})
		It("should return empty if group is not accessible for user", func() {
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: time.Now()}).Error).To(BeNil())

			b, err := m.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(BeEmpty())
			Expect(err).To(BeNil())
		})
		It("should return empty if group member relationship is not exist", func() {
			b, err := m.QueryGroupRole(1, testinfra.BuildSecCtx(types.ID(2), nil))
			Expect(b).To(BeEmpty())
			Expect(err).To(BeNil())
		})
	})
})
