package work_test

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/testinfra"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
)

var _ = Describe("WorkManager", func() {
	var (
		workManager  *work.WorkManager
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		// migration
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		workManager = work.NewWorkManager(testDatabase.DS)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateWork and Detail", func() {
		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			creation := &domain.WorkCreation{Name: "test work", Group: "test group"}
			work, err := workManager.CreateWork(creation)
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
		It("should create new workManager successfully", func() {
			creation := &domain.WorkCreation{Name: "test work", Group: "test group"}
			work, err := workManager.CreateWork(creation)

			Expect(err).To(BeZero())
			Ω(work).ShouldNot(BeZero())
			Ω(work.ID).ShouldNot(BeZero())
			Ω(work.Name).Should(Equal(creation.Name))
			Ω(work.Group).Should(Equal(creation.Group))
			Ω(work.CreateTime).ShouldNot(BeZero())
			Ω(work.Type).Should(Equal(domain.GenericWorkFlow.WorkFlowBase))
			Ω(work.State).Should(Equal(domain.GenericWorkFlow.StateMachine.States[0]))
			//Ω(len(work.Properties)).Should(Equal(0))

			detail, err := workManager.WorkDetail(work.ID)
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
			Expect(detail.Name).To(Equal(creation.Name))
			Expect(detail.Group).To(Equal(creation.Group))
			Expect(detail.CreateTime).ToNot(BeZero())
			Expect(detail.Type).To(Equal(domain.GenericWorkFlow.WorkFlowBase))
			Expect(detail.State).To(Equal(domain.GenericWorkFlow.StateMachine.States[0]))
			Expect(detail.FlowID).To(Equal(domain.GenericWorkFlow.ID))
			Expect(detail.StateName).To(Equal(domain.GenericWorkFlow.StateMachine.States[0].Name))
			//Expect(len(work.Properties)).To(Equal(0))
		})
	})

	Describe("Query All", func() {
		It("should query all works successfully", func() {
			_, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", Group: "test group"})
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "test work2", Group: "test group"})
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork()
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			work1 := (*works)[0]
			Expect(work1.ID).ToNot(BeZero())
			Expect(work1.Name).To(Equal("test work1"))
			Expect(work1.Group).To(Equal("test group"))
			Expect(work1.CreateTime).ToNot(BeZero())
			Expect(work1.FlowID).To(Equal(domain.GenericWorkFlow.ID))
			Expect(work1.StateName).To(Equal(domain.GenericWorkFlow.StateMachine.States[0].Name))
			//Expect(len(work1.Properties)).To(Equal(0))
		})
	})

	Describe("UpdateWork", func() {
		It("should be able to update work", func() {
			detail, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", Group: "test group"})
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(detail.ID, &domain.WorkUpdating{Name: "test work1 new"})
			Expect(err).To(BeZero())
			Expect(updatedWork).ToNot(BeNil())
			Expect(updatedWork.ID).To(Equal(detail.ID))
			Expect(updatedWork.Name).To(Equal("test work1 new"))

			works, err := workManager.QueryWork()
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			Expect((*works)[0].ID).To(Equal(detail.ID))
			Expect((*works)[0].Name).To(Equal("test work1 new"))
		})
		It("should be able to catch error when work not found", func() {
			_, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", Group: "test group"})
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(12345, &domain.WorkUpdating{Name: "test work1 new"})
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
		})

		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			updatedWork, err := workManager.UpdateWork(12345, &domain.WorkUpdating{Name: "test work1 new"})
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})

	Describe("DeleteWork", func() {
		It("should be able to delete work by id", func() {
			_, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", Group: "test group"})
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "test work2", Group: "test group"})
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork()
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			err = workManager.DeleteWork((*works)[0].ID)
			Expect(err).To(BeNil())
			works, err = workManager.QueryWork()
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(1))
		})

		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			err := workManager.DeleteWork(123)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})
})
