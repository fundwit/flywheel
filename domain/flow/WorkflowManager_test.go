package flow_test

import (
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/work"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
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
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}, &flow.WorkStateTransition{}, &domain.WorkProcessStep{}).Error
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
			id, err := manager.CreateWorkStateTransition(&flow.WorkStateTransitionBrief{FlowID: 2}, testinfra.BuildSecCtx(123, []string{}))
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("workflow 2 not found"))
		})
		It("should failed if transition is not acceptable", func() {
			id, err := manager.CreateWorkStateTransition(
				&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "DONE", ToState: "DOING"},
				testinfra.BuildSecCtx(123, []string{}))
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("transition from DONE to DOING is not invalid"))
		})
		It("should failed if update work stateName failed", func() {
			err := testDatabase.DS.GormDB().DropTable(&domain.Work{}).Error
			Expect(err).To(BeNil())

			id, err := manager.CreateWorkStateTransition(
				&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"},
				testinfra.BuildSecCtx(123, []string{}))
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
		It("should failed when work is not exist", func() {
			id, err := manager.CreateWorkStateTransition(
				&flow.WorkStateTransitionBrief{FlowID: 1, WorkID: 1, FromState: "PENDING", ToState: "DOING"},
				testinfra.BuildSecCtx(123, []string{}))
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("record not found"))
		})
		It("should failed when work is forbidden for current user", func() {
			detail := testinfra.BuildWorker(workManager, "test work", types.ID(333), testinfra.BuildSecCtx(types.ID(222), []string{"owner_333"}))
			id, err := manager.CreateWorkStateTransition(
				&flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"},
				testinfra.BuildSecCtx(types.ID(111), []string{}))
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
		It("should failed if stateName is not match fromState", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), []string{"owner_333"})
			detail := testinfra.BuildWorker(workManager, "test work", types.ID(333), sec)

			id, err := manager.CreateWorkStateTransition(
				&flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"}, sec)
			Expect(id).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("expected affected row is 1, but actual is 0"))
		})

		It("should failed if create transition record failed", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), []string{"owner_333"})
			detail := testinfra.BuildWorker(workManager, "test work", types.ID(333), sec)

			Expect(testDatabase.DS.GormDB().DropTable(&flow.WorkStateTransition{}).Error).To(BeNil())

			transition := flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
			id, err := manager.CreateWorkStateTransition(&transition,
				testinfra.BuildSecCtx(types.ID(123), []string{"owner_333"}))
			Expect(id).To(BeZero())
			Expect(err).ToNot(BeZero())
		})

		It("should success when all conditions be satisfied", func() {
			sec := testinfra.BuildSecCtx(types.ID(123), []string{"owner_333"})
			detail := testinfra.BuildWorker(workManager, "test work", types.ID(333), sec)
			creation := flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "PENDING", ToState: "DOING"}
			transition, err := manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())
			Expect(transition).ToNot(BeZero())
			Expect(transition.WorkStateTransitionBrief).To(Equal(creation))
			Expect(transition.ID).ToNot(BeZero())
			Expect(transition.CreateTime).ToNot(BeZero())

			// work.stateName is updated
			detail, err = workManager.WorkDetail(detail.ID, sec)
			Expect(err).To(BeNil())
			Expect(detail.StateName).To(Equal("DOING"))
			Expect(detail.StateBeginTime.UnixNano() >= detail.CreateTime.UnixNano()).To(BeTrue())

			// record is created
			var records []flow.WorkStateTransition
			err = testDatabase.DS.GormDB().Model(&flow.WorkStateTransition{}).Find(&records).Error
			Expect(err).To(BeNil())
			Expect(len(records)).To(Equal(1))
			Expect(records[0].ID).To(Equal(transition.ID))
			Expect(records[0].CreateTime).ToNot(BeZero())
			Expect(records[0].FlowID).To(Equal(creation.FlowID))
			Expect(records[0].WorkID).To(Equal(creation.WorkID))
			Expect(records[0].FromState).To(Equal(creation.FromState))
			Expect(records[0].ToState).To(Equal(creation.ToState))
			Expect(records[0].Creator).To(Equal(sec.Identity.ID))

			handleTime := records[0].CreateTime
			Expect(detail.StateBeginTime).To(Equal(&handleTime))
			Expect(detail.ProcessBeginTime).To(Equal(&handleTime))
			Expect(detail.ProcessEndTime).To(BeNil())

			// should handle process step
			var processSteps []domain.WorkProcessStep
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&processSteps).Error).To(BeNil())
			Expect(processSteps).ToNot(BeNil())
			Expect(len(processSteps)).To(Equal(2))
			Expect(processSteps[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				StateName: creation.FromState, StateCategory: 0, BeginTime: &detail.CreateTime, EndTime: &handleTime}))
			Expect(processSteps[1]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				StateName: creation.ToState, StateCategory: 1, BeginTime: &handleTime, EndTime: nil}))

			// transit to done state: processEndTime should be set
			creation = flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DOING", ToState: "DONE"}
			transition, err = manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())
			detail, err = workManager.WorkDetail(detail.ID, sec)
			Expect(err).To(BeNil())
			Expect(detail.StateBeginTime).ToNot(BeNil())
			Expect(detail.ProcessBeginTime).ToNot(BeNil())
			Expect(detail.ProcessEndTime).ToNot(BeNil())

			// transit back to process state: processEndTime should be reset to nil
			creation = flow.WorkStateTransitionBrief{FlowID: detail.FlowID, WorkID: detail.ID, FromState: "DONE", ToState: "PENDING"}
			transition, err = manager.CreateWorkStateTransition(&creation, sec)
			Expect(err).To(BeNil())
			detail, err = workManager.WorkDetail(detail.ID, sec)
			Expect(err).To(BeNil())
			Expect(detail.StateBeginTime).ToNot(BeNil())
			Expect(detail.ProcessBeginTime).ToNot(BeNil())
			Expect(detail.ProcessEndTime).To(BeNil())
		})
	})
})
