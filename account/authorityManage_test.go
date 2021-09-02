package account_test

import (
	"flywheel/account"
	"flywheel/authority"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/testinfra"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuthorityManage", func() {
	var (
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB().AutoMigrate(&account.User{}, &account.Role{}, &account.Permission{},
			&account.UserRoleBinding{}, &account.RolePermissionBinding{}).Error).To(BeNil())
		account.LoadPermFuncReset()
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("DefaultSecurityConfiguration", func() {
		It("should be able to prepare default security configuration correctly", func() {
			Expect(account.DefaultSecurityConfiguration()).To(BeNil())

			var users []account.User
			var roles []account.Role
			var perms []account.Permission
			var userRoles []account.UserRoleBinding
			var rolePerms []account.RolePermissionBinding

			db := testDatabase.DS.GormDB()
			Expect(db.Find(&users).Error).To(BeNil())
			Expect(len(users)).To(Equal(1))
			Expect(users[0]).To(Equal(account.User{ID: 1, Name: "admin", Secret: account.HashSha256("admin123")}))

			Expect(db.Find(&roles).Error).To(BeNil())
			Expect(len(roles)).To(Equal(1))
			Expect(roles[0]).To(Equal(account.Role{ID: "system-admin", Title: "System Administrator"}))

			Expect(db.Find(&perms).Error).To(BeNil())
			Expect(len(perms)).To(Equal(1))
			Expect(perms[0]).To(Equal(account.Permission{ID: "system:admin", Title: "System Administration"}))

			Expect(db.Find(&userRoles).Error).To(BeNil())
			Expect(len(userRoles)).To(Equal(1))
			Expect(userRoles[0]).To(Equal(account.UserRoleBinding{ID: 1, UserID: 1, RoleID: "system-admin"}))

			Expect(db.Find(&rolePerms).Error).To(BeNil())
			Expect(len(rolePerms)).To(Equal(1))
			Expect(rolePerms[0]).To(Equal(account.RolePermissionBinding{ID: 1, RoleID: "system-admin", PermissionID: "system:admin"}))

			Expect(account.DefaultSecurityConfiguration()).To(BeNil())
			var users1 []account.User
			var roles1 []account.Role
			var perms1 []account.Permission
			var userRoles1 []account.UserRoleBinding
			var rolePerms1 []account.RolePermissionBinding

			Expect(db.Find(&users1).Error).To(BeNil())
			Expect(len(users1)).To(Equal(1))
			Expect(users1[0]).To(Equal(account.User{ID: 1, Name: "admin", Secret: account.HashSha256("admin123")}))

			Expect(db.Find(&roles1).Error).To(BeNil())
			Expect(len(roles1)).To(Equal(1))
			Expect(roles1[0]).To(Equal(account.Role{ID: "system-admin", Title: "System Administrator"}))

			Expect(db.Find(&perms1).Error).To(BeNil())
			Expect(len(perms1)).To(Equal(1))
			Expect(perms1[0]).To(Equal(account.Permission{ID: "system:admin", Title: "System Administration"}))

			Expect(db.Find(&userRoles1).Error).To(BeNil())
			Expect(len(userRoles1)).To(Equal(1))
			Expect(userRoles1[0]).To(Equal(account.UserRoleBinding{ID: 1, UserID: 1, RoleID: "system-admin"}))

			Expect(db.Find(&rolePerms1).Error).To(BeNil())
			Expect(len(rolePerms1)).To(Equal(1))
			Expect(rolePerms1[0]).To(Equal(account.RolePermissionBinding{ID: 1, RoleID: "system-admin", PermissionID: "system:admin"}))
		})
	})

	Describe("LoadPerms", func() {
		It("should return actual project permissions for non system role user", func() {
			now := time.Now()
			Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.ProjectMember{}, &domain.Project{}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Project{ID: 1, Name: "project1", Identifier: "1", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Project{ID: 20, Name: "project20", Identifier: "20", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())

			Expect(testDatabase.DS.GormDB().Create(
				&domain.ProjectMember{ProjectId: 1, MemberId: 3, Role: domain.ProjectRoleManager + "", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.ProjectMember{ProjectId: 10, MemberId: 30, Role: "viewer", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.ProjectMember{ProjectId: 20, MemberId: 3, Role: "viewer", CreateTime: now}).Error).To(BeNil())

			s, gr := account.LoadPermFunc(3)
			Expect(s).To(Equal(authority.Permissions{domain.ProjectRoleManager + "_1", "viewer_20"}))
			Expect(gr).To(Equal(authority.ProjectRoles{{ProjectID: 1, ProjectName: "project1", Role: domain.ProjectRoleManager + "", ProjectIdentifier: "1"},
				{ProjectID: 20, ProjectName: "project20", ProjectIdentifier: "20", Role: "viewer"}}))

			s, gr = account.LoadPermFunc(100)
			Expect(s).To(Equal(authority.Permissions{}))
			Expect(len(gr)).To(Equal(0))
		})

		It("should return aggregated permissions for system role user", func() {
			Expect(account.DefaultSecurityConfiguration()).To(BeNil())
			Expect(testDatabase.DS.GormDB().Save(&account.UserRoleBinding{UserID: 3, RoleID: "system-admin"}).Error).To(BeNil())

			now := time.Now()
			Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.ProjectMember{}, &domain.Project{}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Project{ID: 1, Name: "project1", Identifier: "1", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.Project{ID: 20, Name: "project20", Identifier: "20", NextWorkId: 1, Creator: types.ID(999), CreateTime: now}).Error).To(BeNil())

			Expect(testDatabase.DS.GormDB().Create(
				&domain.ProjectMember{ProjectId: 10, MemberId: 30, Role: "viewer", CreateTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(
				&domain.ProjectMember{ProjectId: 20, MemberId: 3, Role: "viewer", CreateTime: now}).Error).To(BeNil())

			s, gr := account.LoadPermFunc(3)
			Expect(s).To(Equal(authority.Permissions{"system:admin", domain.ProjectRoleManager + "_1", "manager_20"}))
			Expect(gr).To(Equal(authority.ProjectRoles{{ProjectID: 1, ProjectName: "project1", Role: domain.ProjectRoleManager + "", ProjectIdentifier: "1"},
				{ProjectID: 20, ProjectName: "project20", ProjectIdentifier: "20", Role: "manager"}}))

			s, gr = account.LoadPermFunc(1)
			Expect(s).To(Equal(authority.Permissions{"system:admin", domain.ProjectRoleManager + "_1", "manager_20"}))
			Expect(gr).To(Equal(authority.ProjectRoles{{ProjectID: 1, ProjectName: "project1", Role: domain.ProjectRoleManager + "", ProjectIdentifier: "1"},
				{ProjectID: 20, ProjectName: "project20", ProjectIdentifier: "20", Role: "manager"}}))

			s, gr = account.LoadPermFunc(100)
			Expect(s).To(Equal(authority.Permissions{}))
			Expect(len(gr)).To(Equal(0))
		})
	})
})
