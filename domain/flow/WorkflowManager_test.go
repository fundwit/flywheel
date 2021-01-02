package flow_test

import (
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/work"
	"flywheel/testinfra"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
)

var _ = Describe("WorkflowManager", func() {
	var (
		testDatabase *testinfra.TestDatabase
		manager      flow.WorkflowManagerTraits
		workManager  work.WorkManagerTraits
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}, &flow.WorkStateTransition{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		manager = flow.NewWorkflowManager(testDatabase.DS)
		workManager = work.NewWorkManager(testDatabase.DS)
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateWorkStateTransition", func() {
		It("should failed if workflow is not exist", func() {
			id, err := manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 2})
			Expect(id).To(BeNil())
			Expect(err.Error()).To(Equal("workflow 2 not found"))
		})
		It("should failed if transition is not acceptable", func() {
			id, err := manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "DONE", ToState: "DOING"})
			Expect(id).To(BeNil())
			Expect(err.Error()).To(Equal("transition from DONE to DOING is not invalid"))
		})
		It("should failed if update work stateName failed", func() {
			err := testDatabase.DS.GormDB().DropTable(&domain.Work{}).Error
			Expect(err).To(BeNil())

			id, err := manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"})
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
		It("should failed if work not exist or stateName is not match fromState", func() {
			// work is not exist
			id, err := manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"})
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))

			// work stageName is not match fromState
			creation := &domain.WorkCreation{
				Name:  "test work",
				Group: "test group",
			}
			detail, err := workManager.CreateWork(creation)
			Expect(detail).ToNot(BeNil())
			id, err = manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"})
			Expect(id).To(BeNil())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
		})

		It("should failed if create transition record failed", func() {
			creation := &domain.WorkCreation{
				Name:  "test work",
				Group: "test group",
			}
			detail, err := workManager.CreateWork(creation)
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.StateName).To(Equal("PENDING"))

			err = testDatabase.DS.GormDB().DropTable(&flow.WorkStateTransition{}).Error
			Expect(err).To(BeNil())

			transiton := flow.WorkStateTransitionBrief{FlowID: 1, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
			id, err := manager.CreateWorkStateTransition(&transiton)
			Expect(id).To(BeZero())
			Expect(err).ToNot(BeZero())
		})

		It("should success when all conditions be satisfied", func() {
			workCreation := &domain.WorkCreation{
				Name:  "test work",
				Group: "test group",
			}
			detail, err := workManager.CreateWork(workCreation)
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.StateName).To(Equal("PENDING"))

			creation := flow.WorkStateTransitionBrief{FlowID: 1, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
			transition, err := manager.CreateWorkStateTransition(&creation)
			Expect(err).To(BeNil())
			Expect(transition).ToNot(BeZero())
			Expect(transition.WorkStateTransitionBrief).To(Equal(creation))
			Expect(transition.ID).ToNot(BeZero())
			Expect(transition.CreateTime).ToNot(BeZero())

			// work.stateName is updated
			detail, err = workManager.WorkDetail(detail.ID)
			Expect(err).To(BeNil())
			Expect(detail.StateName).To(Equal("DOING"))

			// record is created
			records := []flow.WorkStateTransition{}
			err = testDatabase.DS.GormDB().Model(&flow.WorkStateTransition{}).Find(&records).Error
			Expect(err).To(BeNil())
			Expect(len(records)).To(Equal(1))
			Expect(records[0].ID).To(Equal(transition.ID))
			Expect(records[0].CreateTime).ToNot(BeZero())
			Expect(records[0].FlowID).To(Equal(creation.FlowID))
			Expect(records[0].WorkID).To(Equal(creation.WorkID))
			Expect(records[0].FromState).To(Equal(creation.FromState))
			Expect(records[0].ToState).To(Equal(creation.ToState))
		})
	})
})
