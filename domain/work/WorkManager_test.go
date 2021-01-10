package work_test

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
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

			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1)}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
		It("should create new workManager successfully", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1)}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))

			Expect(err).To(BeZero())
			Expect(work).ToNot(BeZero())
			Expect(work.ID).ToNot(BeZero())
			Expect(work.Name).To(Equal(creation.Name))
			Expect(work.GroupID).To(Equal(creation.GroupID))
			Expect(work.CreateTime).ToNot(BeZero())
			Expect(work.Type).To(Equal(domain.GenericWorkFlow.WorkFlowBase))
			Expect(work.State).To(Equal(domain.GenericWorkFlow.StateMachine.States[0]))

			detail, err := workManager.WorkDetail(work.ID, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
			Expect(detail.Name).To(Equal(creation.Name))
			Expect(detail.GroupID).To(Equal(creation.GroupID))
			Expect(detail.CreateTime).ToNot(BeZero())
			Expect(detail.Type).To(Equal(domain.GenericWorkFlow.WorkFlowBase))
			Expect(detail.State).To(Equal(domain.GenericWorkFlow.StateMachine.States[0]))
			Expect(detail.FlowID).To(Equal(domain.GenericWorkFlow.ID))
			Expect(detail.StateName).To(Equal(domain.GenericWorkFlow.StateMachine.States[0].Name))
			//Expect(len(work.Properties)).To(Equal(0))
		})
		It("should forbid to create to other group", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1)}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_2"}))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
	})

	Describe("CreateWork and Detail", func() {
		It("should forbid to get work detail with permissions", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1)}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			detail, err := workManager.WorkDetail(work.ID, testinfra.BuildSecCtx(200, []string{"owner_2"}))
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
	})

	Describe("Query All", func() {
		It("should query all works successfully", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(2)}, testinfra.BuildSecCtx(2, []string{"owner_2"}))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1", "owner_2"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(0))

			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			work1 := (*works)[0]
			Expect(work1.ID).ToNot(BeZero())
			Expect(work1.Name).To(Equal("test work1"))
			Expect(work1.GroupID).To(Equal(types.ID(1)))
			Expect(work1.CreateTime).ToNot(BeZero())
			Expect(work1.FlowID).To(Equal(domain.GenericWorkFlow.ID))
			Expect(work1.StateName).To(Equal(domain.GenericWorkFlow.StateMachine.States[0].Name))
		})

		It("should query by name and group id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(1)}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(2)}, testinfra.BuildSecCtx(2, []string{"owner_2"}))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(
				&domain.WorkQuery{Name: "work2", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			work1 := (*works)[0]
			Expect(work1.ID).ToNot(BeZero())
			Expect(work1.Name).To(Equal("test work2"))
			Expect(work1.GroupID).To(Equal(types.ID(1)))
			Expect(work1.CreateTime).ToNot(BeZero())
			Expect(work1.FlowID).To(Equal(domain.GenericWorkFlow.ID))
			Expect(work1.StateName).To(Equal(domain.GenericWorkFlow.StateMachine.States[0].Name))
		})
	})

	Describe("UpdateWork", func() {
		It("should be able to update work", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(detail.ID,
				&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			Expect(updatedWork).ToNot(BeNil())
			Expect(updatedWork.ID).To(Equal(detail.ID))
			Expect(updatedWork.Name).To(Equal("test work1 new"))

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			Expect((*works)[0].ID).To(Equal(detail.ID))
			Expect((*works)[0].Name).To(Equal("test work1 new"))
		})
		It("should be able to catch error when work not found", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(404,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("record not found")) // thrown when check permissions
		})

		It("should forbid to update work without permission", func() {
			detail, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(detail.ID,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, []string{"owner_2"}))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			updatedWork, err := workManager.UpdateWork(12345,
				&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})

	Describe("DeleteWork", func() {
		It("should be able to delete work by id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			err = workManager.DeleteWork((*works)[0].ID, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(1))
		})

		It("should forbid to delete without permissions", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(2, []string{"owner_123"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			err := workManager.DeleteWork(123, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})
})
