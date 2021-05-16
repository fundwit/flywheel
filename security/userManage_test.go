package security_test

import (
	"flywheel/bizerror"
	"flywheel/persistence"
	"flywheel/security"
	"flywheel/testinfra"
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
		Expect(testDatabase.DS.GormDB().AutoMigrate(&security.User{}).Error).To(BeNil())
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("UpdateBasicAuthSecret", func() {
		It("should be able to update basic auth secret correctly", func() {
			sec := security.Context{Identity: security.Identity{ID: 1}}
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "aaa", Secret: security.HashSha256("123456")}).Error).To(BeNil())
			Expect(security.UpdateBasicAuthSecret(&security.BasicAuthUpdating{OriginalSecret: "234567", NewSecret: "654321"}, &sec)).To(Equal(bizerror.ErrInvalidPassword))
			Expect(security.UpdateBasicAuthSecret(&security.BasicAuthUpdating{OriginalSecret: "123456", NewSecret: "654321"}, &sec)).To(BeNil())

			user := security.User{}
			Expect(testDatabase.DS.GormDB().Model(&security.User{}).Where(&security.User{ID: sec.Identity.ID}).First(&user).Error).To(BeNil())
			Expect(user.Secret).To(Equal(security.HashSha256("654321")))
		})
	})

	Describe("QueryUsers", func() {
		It("should be blocked when user lack of permission", func() {
			sec := security.Context{Identity: security.Identity{ID: 1}}
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "aaa", Secret: security.HashSha256("123456")}).Error).To(BeNil())

			users, err := security.QueryUsers(&sec)
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(users).To(BeNil())
		})

		It("should be able to query users correctly", func() {
			sec := security.Context{Identity: security.Identity{ID: 1}, Perms: []string{security.SystemAdminPermission.ID}}
			Expect(testDatabase.DS.GormDB().Save(&security.User{ID: 1, Name: "aaa", Secret: security.HashSha256("123456")}).Error).To(BeNil())

			users, err := security.QueryUsers(&sec)
			Expect(err).To(BeNil())
			Expect(len(*users)).To(Equal(1))
			Expect((*users)[0]).To(Equal(security.UserInfo{ID: 1, Name: "aaa"}))
		})
	})

	Describe("CreateUser", func() {
		It("should be blocked when user lack of permission", func() {
			sec := &security.Context{Identity: security.Identity{ID: 1}}

			u, err := security.CreateUser(&security.UserCreation{Name: "test", Secret: "123456"}, sec)
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(u).To(BeNil())
		})

		It("should be able to create users correctly", func() {
			sec := &security.Context{Identity: security.Identity{ID: 1}, Perms: []string{security.SystemAdminPermission.ID}}
			u, err := security.CreateUser(&security.UserCreation{Name: "test", Secret: "123456"}, sec)
			Expect(err).To(BeNil())
			Expect((*u).ID).ToNot(BeZero())
			Expect(*u).To(Equal(security.UserInfo{ID: u.ID, Name: "test"}))

			user := security.User{}
			Expect(testDatabase.DS.GormDB().Model(&security.User{}).Where(&security.User{ID: u.ID}).First(&user).Error).To(BeNil())
			Expect(user).To(Equal(security.User{ID: u.ID, Name: "test", Secret: security.HashSha256("123456")}))
		})
	})
})
