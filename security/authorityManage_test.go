package security_test

import (
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("AuthorityManage", func() {
	var (
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB().AutoMigrate(&security.User{}, &security.Role{}, &security.Permission{},
			&security.UserRoleBinding{}, &security.RolePermissionBinding{}).Error).To(BeNil())
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("DefaultSecurityConfiguration", func() {
		It("should be able to prepare default security configuration correctly", func() {
			Expect(security.DefaultSecurityConfiguration()).To(BeNil())

			var users []security.User
			var roles []security.Role
			var perms []security.Permission
			var userRoles []security.UserRoleBinding
			var rolePerms []security.RolePermissionBinding

			db := testDatabase.DS.GormDB()
			Expect(db.Find(&users).Error).To(BeNil())
			Expect(len(users)).To(Equal(1))
			Expect(users[0]).To(Equal(security.User{ID: 1, Name: "admin", Secret: security.HashSha256("admin123")}))

			Expect(db.Find(&roles).Error).To(BeNil())
			Expect(len(roles)).To(Equal(1))
			Expect(roles[0]).To(Equal(security.Role{ID: "system-admin", Title: "System Administrator"}))

			Expect(db.Find(&perms).Error).To(BeNil())
			Expect(len(perms)).To(Equal(1))
			Expect(perms[0]).To(Equal(security.Permission{ID: "system:admin", Title: "System Administration"}))

			Expect(db.Find(&userRoles).Error).To(BeNil())
			Expect(len(userRoles)).To(Equal(1))
			Expect(userRoles[0]).To(Equal(security.UserRoleBinding{ID: 1, UserID: 1, RoleID: "system-admin"}))

			Expect(db.Find(&rolePerms).Error).To(BeNil())
			Expect(len(rolePerms)).To(Equal(1))
			Expect(rolePerms[0]).To(Equal(security.RolePermissionBinding{ID: 1, RoleID: "system-admin", PermissionID: "system:admin"}))

			Expect(security.DefaultSecurityConfiguration()).To(BeNil())
			var users1 []security.User
			var roles1 []security.Role
			var perms1 []security.Permission
			var userRoles1 []security.UserRoleBinding
			var rolePerms1 []security.RolePermissionBinding

			Expect(db.Find(&users1).Error).To(BeNil())
			Expect(len(users1)).To(Equal(1))
			Expect(users1[0]).To(Equal(security.User{ID: 1, Name: "admin", Secret: security.HashSha256("admin123")}))

			Expect(db.Find(&roles1).Error).To(BeNil())
			Expect(len(roles1)).To(Equal(1))
			Expect(roles1[0]).To(Equal(security.Role{ID: "system-admin", Title: "System Administrator"}))

			Expect(db.Find(&perms1).Error).To(BeNil())
			Expect(len(perms1)).To(Equal(1))
			Expect(perms1[0]).To(Equal(security.Permission{ID: "system:admin", Title: "System Administration"}))

			Expect(db.Find(&userRoles1).Error).To(BeNil())
			Expect(len(userRoles1)).To(Equal(1))
			Expect(userRoles1[0]).To(Equal(security.UserRoleBinding{ID: 1, UserID: 1, RoleID: "system-admin"}))

			Expect(db.Find(&rolePerms1).Error).To(BeNil())
			Expect(len(rolePerms1)).To(Equal(1))
			Expect(rolePerms1[0]).To(Equal(security.RolePermissionBinding{ID: 1, RoleID: "system-admin", PermissionID: "system:admin"}))
		})
	})

	Describe("LoadPerms", func() {
		It("should return actual permissions when matched", func() {
			now := time.Now()
			Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.GroupMember{}, &domain.Group{}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 1, Name: "group1", Identifier: "1", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 20, Name: "group20", Identifier: "20", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())

			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 10, MemberId: 30, Role: "viewer", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 20, MemberId: 3, Role: "viewer", CreateTime: now}).Error).To(BeNil())

			s, gr := security.LoadPerms(3)
			Expect(len(s)).To(Equal(2))
			Expect(s).To(Equal([]string{"owner_1", "viewer_20"}))
			Expect(gr).To(Equal([]domain.GroupRole{{GroupID: 1, GroupName: "group1", Role: "owner", GroupIdentifier: "1"},
				{GroupID: 20, GroupName: "group20", GroupIdentifier: "20", Role: "viewer"}}))

			s, gr = security.LoadPerms(100)
			Expect(len(s)).To(Equal(0))
			Expect(len(gr)).To(Equal(0))
		})

		It("should return actual permissions with system permissions", func() {
			Expect(security.DefaultSecurityConfiguration()).To(BeNil())
			Expect(testDatabase.DS.GormDB().Save(&security.UserRoleBinding{UserID: 3, RoleID: "system-admin"}).Error).To(BeNil())

			now := time.Now()
			Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.GroupMember{}, &domain.Group{}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 1, Name: "group1", Identifier: "1", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Group{ID: 20, Name: "group20", Identifier: "20", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())

			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 1, MemberId: 3, Role: "owner", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 10, MemberId: 30, Role: "viewer", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.GroupMember{GroupID: 20, MemberId: 3, Role: "viewer", CreateTime: now}).Error).To(BeNil())

			s, gr := security.LoadPerms(3)
			Expect(len(s)).To(Equal(3))
			Expect(s).To(Equal([]string{"system:admin", "owner_1", "viewer_20"}))
			Expect(gr).To(Equal([]domain.GroupRole{{GroupID: 1, GroupName: "group1", Role: "owner", GroupIdentifier: "1"},
				{GroupID: 20, GroupName: "group20", GroupIdentifier: "20", Role: "viewer"}}))

			s, gr = security.LoadPerms(1)
			Expect(len(s)).To(Equal(1))
			Expect(s).To(Equal([]string{"system:admin"}))
			Expect(len(gr)).To(Equal(0))

			s, gr = security.LoadPerms(100)
			Expect(len(s)).To(Equal(0))
			Expect(len(gr)).To(Equal(0))
		})
	})
})