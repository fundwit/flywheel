package domain_test

import (
	"flywheel/domain"
	"flywheel/domain/worktype"
	"flywheel/testinfra"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
)

var _ = Describe("WorkManager", func() {
	var (
		workManager  *domain.WorkManager
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		// migration
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		workManager = domain.NewWorkManager(testDatabase.DS)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateWork and Detail", func() {
		It("should create new workManager successfully", func() {
			creation := &domain.WorkCreation{
				Name:  "test work",
				Group: "test group",
			}
			work, err := workManager.CreateWork(creation)

			Expect(err).To(BeZero())
			Ω(work).ShouldNot(BeZero())
			Ω(work.ID).ShouldNot(BeZero())
			Ω(work.Name).Should(Equal(creation.Name))
			Ω(work.Group).Should(Equal(creation.Group))
			Ω(work.CreateTime).ShouldNot(BeZero())
			Ω(work.Type).Should(Equal(worktype.GenericWorkType.WorkTypeBase))
			Ω(work.State).Should(Equal(worktype.GenericWorkType.StateMachine.States[0]))
			//Ω(len(work.Properties)).Should(Equal(0))

			detail, err := workManager.WorkDetail(work.ID)
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
			Expect(detail.Name).To(Equal(creation.Name))
			Expect(detail.Group).To(Equal(creation.Group))
			Expect(detail.CreateTime).ToNot(BeZero())
			Expect(detail.Type).To(Equal(worktype.GenericWorkType.WorkTypeBase))
			Expect(detail.State).To(Equal(worktype.GenericWorkType.StateMachine.States[0]))
			Expect(detail.TypeID).To(Equal(worktype.GenericWorkType.ID))
			Expect(detail.StateName).To(Equal(worktype.GenericWorkType.StateMachine.States[0].Name))
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
			Expect(work1.TypeID).To(Equal(worktype.GenericWorkType.ID))
			Expect(work1.StateName).To(Equal(worktype.GenericWorkType.StateMachine.States[0].Name))
			//Expect(len(work1.Properties)).To(Equal(0))
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
	})
})
