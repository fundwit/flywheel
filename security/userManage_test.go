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
	BeforeSuite(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
	})
	AfterSuite(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})
	BeforeEach(func() {
		persistence.ActiveDataSourceManager = testDatabase.DS
		Expect(testDatabase.DS.GormDB().AutoMigrate(&security.User{}).Error).To(BeNil())
	})
	AfterEach(func() {
		Expect(testDatabase.DS.GormDB().DropTable(&security.User{}).Error).To(BeNil())
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
})
