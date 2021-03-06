package work_test

import (
	"errors"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/testinfra"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
	"time"
)

var _ = Describe("WorkManager", func() {
	var (
		flowManager  *flow.WorkflowManager
		workManager  *work.WorkManager
		testDatabase *testinfra.TestDatabase
		flowDetail   *domain.WorkflowDetail
		flowDetail2  *domain.WorkflowDetail
	)
	BeforeSuite(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
	})
	AfterSuite(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})
	BeforeEach(func() {
		err := testDatabase.DS.GormDB().AutoMigrate(&domain.Work{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error
		if err != nil {
			log.Fatalf("database migration failed %v\n", err)
		}
		flowManager = flow.NewWorkflowManager(testDatabase.DS)
		creation := &flow.WorkflowCreation{Name: "test workflow1", GroupID: types.ID(1), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		flowDetail, err = flowManager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
		Expect(err).To(BeNil())
		flowDetail.CreateTime = flowDetail.CreateTime.Round(time.Millisecond)
		creation = &flow.WorkflowCreation{Name: "test workflow2", GroupID: types.ID(2), StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		flowDetail2, err = flowManager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, []string{"owner_2"}))
		flowDetail2.CreateTime = flowDetail2.CreateTime.Round(time.Millisecond)
		Expect(err).To(BeNil())

		workManager = work.NewWorkManager(testDatabase.DS, flowManager)
	})
	AfterEach(func() {
		err := testDatabase.DS.GormDB().DropTable(&domain.Work{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error
		if err != nil {
			log.Printf("database migration failed %v\n", err)
		}
	})

	Describe("CreateWork", func() {
		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1), FlowID: flowDetail.ID}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
		It("should create new work successfully", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1), FlowID: flowDetail.ID}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))

			Expect(err).To(BeZero())
			Expect(work).ToNot(BeZero())
			Expect(work.ID).ToNot(BeZero())
			Expect(work.Name).To(Equal(creation.Name))
			Expect(work.GroupID).To(Equal(creation.GroupID))
			Expect(work.CreateTime.Sub(time.Now()) < time.Minute).To(BeTrue())
			Expect(work.FlowID).To(Equal(flowDetail.ID))
			Expect(work.OrderInState).To(Equal(work.CreateTime.UnixNano() / 1e6))
			Expect(work.Type).To(Equal(flowDetail.Workflow))
			Expect(work.State).To(Equal(flowDetail.StateMachine.States[0]))
			Expect(work.StateBeginTime).To(Equal(&work.CreateTime))
			Expect(work.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))

			detail, err := workManager.WorkDetail(work.ID, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
			Expect(detail.Name).To(Equal(creation.Name))
			Expect(detail.GroupID).To(Equal(creation.GroupID))
			Expect(detail.CreateTime.Sub(time.Now()) < time.Minute).To(BeTrue())
			Expect(detail.Type).To(Equal(flowDetail.Workflow))
			Expect(detail.State).To(Equal(flowDetail.StateMachine.States[0]))
			Expect(detail.FlowID).To(Equal(flowDetail.ID))
			Expect(detail.OrderInState).To(Equal(work.CreateTime.UnixNano() / 1e6))
			Expect(detail.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
			//Expect(len(work.Properties)).To(Equal(0))

			// should create init process step
			var initProcessStep []domain.WorkProcessStep
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&initProcessStep).Error).To(BeNil())
			Expect(initProcessStep).ToNot(BeNil())
			Expect(len(initProcessStep)).To(Equal(1))
			Expect(initProcessStep[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				StateName: detail.StateName, StateCategory: detail.State.Category, BeginTime: detail.CreateTime, EndTime: nil}))
		})
		It("should forbid to create to other group", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1), FlowID: flowDetail.ID}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_2"}))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
	})

	Describe("DetailWork", func() {
		It("should forbid to get work detail with permissions", func() {
			creation := &domain.WorkCreation{Name: "test work", GroupID: types.ID(1), FlowID: flowDetail.ID}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, []string{"owner_1"}))
			Expect(err).To(BeNil())

			detail, err := workManager.WorkDetail(work.ID, testinfra.BuildSecCtx(200, []string{"owner_2"}))
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
		It("should return error when work not found", func() {
			detail, err := workManager.WorkDetail(types.ID(404), testinfra.BuildSecCtx(200, []string{"owner_2"}))
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(gorm.ErrRecordNotFound.Error()))
		})
		It("should return error when workflow not found", func() {
			// TODO
		})
		It("should return error when state is invalid", func() {
			// TODO
		})
	})

	Describe("Query All", func() {
		It("should query all works successfully", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(2), FlowID: flowDetail2.ID}, testinfra.BuildSecCtx(2, []string{"owner_2"}))
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
			Expect(work1.FlowID).To(Equal(flowDetail.ID))
			Expect(work1.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work1.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
		})

		It("should query by name and group id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(2), FlowID: flowDetail2.ID}, testinfra.BuildSecCtx(2, []string{"owner_2"}))
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
			Expect(work1.FlowID).To(Equal(flowDetail.ID))
			Expect(work1.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work1.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
			Expect(work1.State).To(Equal(flowDetail.StateMachine.States[0]))
		})

		It("should query by stateCategory", func() {
			work1, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			work2, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			Expect(testDatabase.DS.GormDB().Model(&domain.Work{}).Where(&domain.Work{ID: work2.ID}).
				Update("state_category", state.InProcess).Error).To(BeNil())

			work3, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work3", GroupID: types.ID(1), FlowID: flowDetail.ID}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			Expect(testDatabase.DS.GormDB().Model(&domain.Work{}).Where(&domain.Work{ID: work3.ID}).
				Update("state_category", state.Done).Error).To(BeNil())

			works, err := workManager.QueryWork(&domain.WorkQuery{GroupID: types.ID(1), StateCategories: []state.Category{state.InBacklog, state.InProcess}},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))
			Expect((*works)[0].ID).To(Equal(work1.ID))
			Expect((*works)[1].ID).To(Equal(work2.ID))
		})

		It("works should be ordered by orderInState asc and id asc", func() {
			now := time.Now()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", GroupID: 1,
				CreateTime: time.Now(), FlowID: flowDetail.ID, OrderInState: 2, StateName: "PENDING", StateBeginTime: &now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 1, Name: "w2", GroupID: 1,
				CreateTime: time.Now(), FlowID: flowDetail.ID, OrderInState: 2, StateName: "PENDING", StateBeginTime: &now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 3, Name: "w3", GroupID: 1,
				CreateTime: time.Now(), FlowID: flowDetail.ID, OrderInState: 1, StateName: "PENDING", StateBeginTime: &now}).Error).To(BeNil())

			// order by orderInState:    w3(1) > w2(2) = w1(2)
			// order by id (default):         w2(1) > w1(2)
			works, err := workManager.QueryWork(&domain.WorkQuery{GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(3))
			Expect((*works)[0].Name).To(Equal("w3"))
			Expect((*works)[1].Name).To(Equal("w2"))
			Expect((*works)[2].Name).To(Equal("w1"))
		})

		It("should return error if failed to find state", func() {
			now := time.Now()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", GroupID: 1,
				CreateTime: time.Now(), FlowID: flowDetail.ID, OrderInState: 2, StateName: "UNKNOWN", StateBeginTime: &now}).Error).To(BeNil())
			works, err := workManager.QueryWork(&domain.WorkQuery{GroupID: types.ID(1)},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(works).To(BeNil())
			Expect(err.Error()).To(Equal("invalid state"))
		})
	})

	Describe("UpdateWork", func() {
		It("should be able to update work", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			updatedWork, err := workManager.UpdateWork(detail.ID,
				&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			Expect(updatedWork).ToNot(BeNil())
			Expect(updatedWork.ID).To(Equal(detail.ID))
			Expect(updatedWork.Name).To(Equal("test work1 new"))
			Expect(updatedWork.State).To(Equal(flowDetail.StateMachine.States[0]))

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			Expect((*works)[0].ID).To(Equal(detail.ID))
			Expect((*works)[0].Name).To(Equal("test work1 new"))
			Expect((*works)[0].State).To(Equal(flowDetail.StateMachine.States[0]))
		})
		It("should be able to catch error when work not found", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
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
			detail, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
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

		It("should return error if failed to find state", func() {
			now := time.Now()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", GroupID: 1,
				CreateTime: time.Now(), FlowID: flowDetail.ID, OrderInState: 2, StateName: "UNKNOWN", StateBeginTime: &now}).Error).To(BeNil())
			updatedWork, err := workManager.UpdateWork(2,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(updatedWork).To(BeNil())
			Expect(err.Error()).To(Equal("invalid state"))
		})
	})

	Describe("DeleteWork", func() {
		It("should be able to delete work by id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", GroupID: types.ID(1), FlowID: flowDetail.ID},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			testDatabase.DS.GormDB().AutoMigrate(&domain.WorkStateTransition{})
			err = testDatabase.DS.GormDB().Create(&domain.WorkStateTransition{ID: 1, CreateTime: time.Now(), Creator: 1,
				WorkStateTransitionBrief: domain.WorkStateTransitionBrief{FlowID: 1, WorkID: (*works)[0].ID, FromState: "PENDING", ToState: "DOING"}}).Error
			Expect(err).To(BeNil())
			err = testDatabase.DS.GormDB().Create(&domain.WorkStateTransition{ID: 2, CreateTime: time.Now(), Creator: 1,
				WorkStateTransitionBrief: domain.WorkStateTransitionBrief{FlowID: 1, WorkID: 2, FromState: "PENDING", ToState: "DOING"}}).Error
			Expect(err).To(BeNil())
			transition := domain.WorkStateTransition{}
			Expect(testDatabase.DS.GormDB().First(&transition, domain.WorkStateTransition{ID: 1}).Error).To(BeNil())
			Expect(transition.WorkID).To(Equal((*works)[0].ID))
			transition = domain.WorkStateTransition{}
			Expect(testDatabase.DS.GormDB().First(&transition, domain.WorkStateTransition{ID: 2}).Error).To(BeNil())
			Expect(transition.WorkID).To(Equal(types.ID(2)))

			// do delete work
			workIdToDelete := (*works)[0].ID
			err = workManager.DeleteWork(workIdToDelete, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(1))

			// transitions should also be deleted
			// transition of id 1 was deleted
			transition = domain.WorkStateTransition{}
			Expect(testDatabase.DS.GormDB().First(&transition, domain.WorkStateTransition{ID: 1}).Error).To(Equal(gorm.ErrRecordNotFound))
			// transition of id 2 still remains
			transition = domain.WorkStateTransition{}
			Expect(testDatabase.DS.GormDB().First(&transition, domain.WorkStateTransition{ID: 2}).Error).To(BeNil())
			Expect(transition.WorkID).To(Equal(types.ID(2)))

			// work process steps should also be deleted
			processStep := domain.WorkProcessStep{}
			Expect(testDatabase.DS.GormDB().First(&processStep, domain.WorkProcessStep{WorkID: workIdToDelete}).Error).To(Equal(gorm.ErrRecordNotFound))
			processStep = domain.WorkProcessStep{}
			Expect(testDatabase.DS.GormDB().First(&processStep).Error).To(BeNil())
		})

		It("should forbid to delete without permissions", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(2, []string{"owner_123"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be able to catch db errors", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", GroupID: types.ID(1), FlowID: flowDetail.ID},
				testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).To(BeZero())

			Expect(testDatabase.DS.GormDB().DropTable(&domain.WorkStateTransition{}).Error).To(BeNil())
			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_state_transitions' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, []string{"owner_1"}))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})

	Describe("UpdateStateRangeOrders", func() {
		It("should do nothing when input is empty", func() {
			Expect(workManager.UpdateStateRangeOrders(nil, nil)).To(BeNil())
			Expect(workManager.UpdateStateRangeOrders(&[]domain.StageRangeOrderUpdating{}, nil)).To(BeNil())
		})

		It("should be able to handle forbidden access", func() {
			Expect(workManager.UpdateStateRangeOrders(&[]domain.StageRangeOrderUpdating{
				{ID: 1, NewOlder: 3, OldOlder: 2}}, nil)).To(Equal(errors.New("record not found")))
			Expect(workManager.UpdateStateRangeOrders(&[]domain.StageRangeOrderUpdating{
				{ID: 1, NewOlder: 3, OldOlder: 2}}, testinfra.BuildSecCtx(1, []string{"owner_404"}))).To(Equal(errors.New("record not found")))
		})

		It("should update order", func() {
			secCtx := testinfra.BuildSecCtx(1, []string{"owner_1"})
			_, err := workManager.CreateWork(&domain.WorkCreation{Name: "w1", GroupID: types.ID(1), FlowID: flowDetail.ID}, secCtx)
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "w2", GroupID: types.ID(1), FlowID: flowDetail.ID}, secCtx)
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "w3", GroupID: types.ID(1), FlowID: flowDetail.ID}, secCtx)
			Expect(err).To(BeZero())

			// default w1 > w2 > w3
			listPtr, err := workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list := *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

			// invalid data
			Expect(workManager.UpdateStateRangeOrders(&[]domain.StageRangeOrderUpdating{{ID: list[0].ID, NewOlder: 3, OldOlder: 2}}, secCtx)).
				To(Equal(errors.New("expected affected row is 1, but actual is 0")))

			listPtr, err = workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list = *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

			// valid data: w3 > w2 > w1
			Expect(workManager.UpdateStateRangeOrders(&[]domain.StageRangeOrderUpdating{
				{ID: list[0].ID, NewOlder: list[2].OrderInState + 2, OldOlder: list[0].OrderInState},
				{ID: list[1].ID, NewOlder: list[2].OrderInState + 1, OldOlder: list[1].OrderInState}}, secCtx)).To(BeNil())

			listPtr, err = workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list = *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w3", "w2", "w1"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())
		})
	})
})
