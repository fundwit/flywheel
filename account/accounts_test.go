package account_test

import (
	"context"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("userManage", func() {
	var (
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB(context.TODO()).AutoMigrate(&account.User{}).Error).To(BeNil())
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("UpdateBasicAuthSecret", func() {
		It("should be able to update basic auth secret correctly", func() {
			sec := session.Session{Identity: session.Identity{ID: 1}}
			Expect(testDatabase.DS.GormDB(context.TODO()).Save(&account.User{ID: 1, Name: "aaa", Secret: account.HashSha256("123456")}).Error).To(BeNil())
			Expect(account.UpdateBasicAuthSecret(&account.BasicAuthUpdating{OriginalSecret: "234567", NewSecret: "654321"}, &sec)).To(Equal(bizerror.ErrInvalidPassword))
			Expect(account.UpdateBasicAuthSecret(&account.BasicAuthUpdating{OriginalSecret: "123456", NewSecret: "654321"}, &sec)).To(BeNil())

			user := account.User{}
			Expect(testDatabase.DS.GormDB(context.TODO()).Model(&account.User{}).Where(&account.User{ID: sec.Identity.ID}).First(&user).Error).To(BeNil())
			Expect(user.Secret).To(Equal(account.HashSha256("654321")))
		})
	})

	Describe("DisplayName", func() {
		It("should be able to compute display name", func() {
			Expect(account.User{Name: "test", Nickname: "Test"}.DisplayName()).To(Equal("Test"))
			Expect(account.User{Name: "test", Nickname: ""}.DisplayName()).To(Equal("test"))
			Expect(account.User{Name: "test"}.DisplayName()).To(Equal("test"))

			Expect(account.UserInfo{Name: "test", Nickname: "Test"}.DisplayName()).To(Equal("Test"))
			Expect(account.UserInfo{Name: "test", Nickname: ""}.DisplayName()).To(Equal("test"))
			Expect(account.UserInfo{Name: "test"}.DisplayName()).To(Equal("test"))
		})
	})

	Describe("QueryUsers", func() {
		It("should be able to query users correctly", func() {
			sec := session.Session{Identity: session.Identity{ID: 1}, Perms: []string{account.SystemAdminPermission.ID}}
			Expect(testDatabase.DS.GormDB(context.TODO()).Save(&account.User{ID: 1, Name: "aaa", Secret: account.HashSha256("123456")}).Error).To(BeNil())

			users, err := account.QueryUsers(&sec)
			Expect(err).To(BeNil())
			Expect(len(*users)).To(Equal(1))
			Expect((*users)[0]).To(Equal(account.UserInfo{ID: 1, Name: "aaa"}))
		})
	})

	Describe("CreateUser", func() {
		It("should be blocked when user lack of permission", func() {
			sec := &session.Session{Identity: session.Identity{ID: 1}}

			u, err := account.CreateUser(&account.UserCreation{Name: "test", Secret: "123456"}, sec)
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(u).To(BeNil())
		})

		It("should be able to create users correctly", func() {
			sec := &session.Session{Identity: session.Identity{ID: 1}, Perms: []string{account.SystemAdminPermission.ID}}
			u, err := account.CreateUser(&account.UserCreation{Name: "test", Secret: "123456"}, sec)
			Expect(err).To(BeNil())
			Expect((*u).ID).ToNot(BeZero())
			Expect(*u).To(Equal(account.UserInfo{ID: u.ID, Name: "test"}))

			user := account.User{}
			Expect(testDatabase.DS.GormDB(context.TODO()).Model(&account.User{}).Where(&account.User{ID: u.ID}).First(&user).Error).To(BeNil())
			Expect(user).To(Equal(account.User{ID: u.ID, Name: "test", Secret: account.HashSha256("123456")}))
		})

		It("should be able to create users with nickname correctly", func() {
			sec := &session.Session{Identity: session.Identity{ID: 1}, Perms: []string{account.SystemAdminPermission.ID}}
			u, err := account.CreateUser(&account.UserCreation{Name: "test", Nickname: "Test User", Secret: "123456"}, sec)
			Expect(err).To(BeNil())
			Expect((*u).ID).ToNot(BeZero())
			Expect(*u).To(Equal(account.UserInfo{ID: u.ID, Name: "test", Nickname: "Test User"}))

			user := account.User{}
			Expect(testDatabase.DS.GormDB(context.TODO()).Model(&account.User{}).Where(&account.User{ID: u.ID}).First(&user).Error).To(BeNil())
			Expect(user).To(Equal(account.User{ID: u.ID, Name: "test", Nickname: "Test User", Secret: account.HashSha256("123456")}))
		})
	})

	Describe("UpdateUser", func() {
		It("should be able to update nickname correctly", func() {
			sec := session.Session{Identity: session.Identity{ID: 1}}
			Expect(testDatabase.DS.GormDB(context.TODO()).Save(&account.User{ID: 1, Name: "aaa", Secret: account.HashSha256("123456")}).Error).To(BeNil())

			Expect(account.UpdateUser(404, &account.UserUpdation{Nickname: "New Name"}, &sec)).To(Equal(bizerror.ErrForbidden))
			Expect(account.UpdateUser(2, &account.UserUpdation{Nickname: "New Name"},
				&session.Session{Identity: session.Identity{ID: 2}})).To(Equal(gorm.ErrRecordNotFound))

			Expect(account.UpdateUser(1, &account.UserUpdation{Nickname: "New Name 1"}, &sec)).To(BeNil())
			user := account.User{}
			Expect(testDatabase.DS.GormDB(context.TODO()).Model(&account.User{}).Where(&account.User{ID: 1}).First(&user).Error).To(BeNil())
			Expect(user.Nickname).To(Equal("New Name 1"))

			Expect(account.UpdateUser(1, &account.UserUpdation{Nickname: "New Name 2"},
				&session.Session{Perms: authority.Permissions{account.SystemAdminPermission.ID}, Identity: session.Identity{ID: 404}})).To(BeNil())
			user = account.User{}
			Expect(testDatabase.DS.GormDB(context.TODO()).Model(&account.User{}).Where(&account.User{ID: 1}).First(&user).Error).To(BeNil())
			Expect(user.Nickname).To(Equal("New Name 2"))
		})
	})

	Describe("QueryAccountNames", func() {
		It("should return correct account names", func() {
			s := &session.Session{Identity: session.Identity{ID: 1}}
			ret, err := account.QueryAccountNames(nil, s)
			Expect(err).To(BeNil())
			Expect(len(ret)).To(BeZero())

			ret, err = account.QueryAccountNames([]types.ID{}, s)
			Expect(err).To(BeNil())
			Expect(len(ret)).To(BeZero())

			db := testDatabase.DS.GormDB(context.TODO())
			Expect(db.Save(&account.User{ID: 1, Name: "u1", Secret: "xxx"}).Error).To(BeNil())
			Expect(db.Save(&account.User{ID: 2, Name: "u2", Nickname: "User2", Secret: "xxx"}).Error).To(BeNil())
			Expect(db.Save(&account.User{ID: 3, Name: "u3", Secret: "xxx"}).Error).To(BeNil())

			ret, err = account.QueryAccountNames([]types.ID{1, 2, 4}, s)
			Expect(err).To(BeNil())
			Expect(len(ret)).To(Equal(2))
			Expect(ret).To(Equal(map[types.ID]string{1: "u1", 2: "User2"}))
		})
	})
})
