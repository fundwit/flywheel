package work_test

import (
	"errors"
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/testinfra"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkManager", func() {
	var (
		flowManager  *flow.WorkflowManager
		workManager  *work.WorkManager
		testDatabase *testinfra.TestDatabase
		flowDetail   *domain.WorkflowDetail
		flowDetail2  *domain.WorkflowDetail
		project1     *domain.Project
		project2     *domain.Project
		lastEvents   []event.EventRecord
	)
	BeforeSuite(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
	})
	AfterSuite(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})
	BeforeEach(func() {
		Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error).To(BeNil())

		persistence.ActiveDataSourceManager = testDatabase.DS
		var err error
		project1, err = namespace.CreateProject(&domain.ProjectCreating{Name: "project 1", Identifier: "GR1"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", account.SystemAdminPermission.ID))
		Expect(err).To(BeNil())
		project2, err = namespace.CreateProject(&domain.ProjectCreating{Name: "project 2", Identifier: "GR2"},
			testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_2", account.SystemAdminPermission.ID))
		Expect(err).To(BeNil())

		flowManager = flow.NewWorkflowManager(testDatabase.DS)
		creation := &flow.WorkflowCreation{Name: "test workflow1", ProjectID: project1.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		flowDetail, err = flowManager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
		Expect(err).To(BeNil())
		flowDetail.CreateTime = flowDetail.CreateTime.Round(time.Millisecond)

		creation = &flow.WorkflowCreation{Name: "test workflow2", ProjectID: project2.ID, StateMachine: domain.GenericWorkflowTemplate.StateMachine}
		flowDetail2, err = flowManager.CreateWorkflow(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String()))
		Expect(err).To(BeNil())
		flowDetail2.CreateTime = flowDetail2.CreateTime.Round(time.Millisecond)

		workManager = work.NewWorkManager(testDatabase.DS, flowManager)

		lastEvents = []event.EventRecord{}
		event.EventPersistCreateFunc = func(record *event.EventRecord, db *gorm.DB) error {
			lastEvents = append(lastEvents, *record)
			return nil
		}
	})
	AfterEach(func() {
		err := testDatabase.DS.GormDB().DropTable(&domain.Project{}, &domain.ProjectMember{}, &domain.Work{}, &domain.WorkProcessStep{},
			&domain.Workflow{}, &domain.WorkflowState{}, &domain.WorkflowStateTransition{}).Error
		if err != nil {
			log.Printf("database migration failed %v\n", err)
		}
	})

	Describe("CreateWork", func() {
		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
			Expect(len(lastEvents)).To(BeZero())
		})

		It("should failed when initial state is unknown", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: "UNKNOWN"}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal(bizerror.ErrUnknownState.Error()))
			Expect(len(lastEvents)).To(BeZero())
		})

		It("should create new work successfully", func() {
			creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
			sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
			work, err := workManager.CreateWork(creation, sec)

			Expect(err).To(BeZero())
			Expect(work).ToNot(BeZero())
			Expect(work.ID).ToNot(BeZero())
			Expect(work.Identifier).ToNot(BeZero())
			Expect(work.Name).To(Equal(creation.Name))
			Expect(work.ProjectID).To(Equal(creation.ProjectID))
			Expect(work.CreateTime.Time().Sub(time.Now()) < time.Minute).To(BeTrue())
			Expect(work.FlowID).To(Equal(flowDetail.ID))
			Expect(work.OrderInState).To(Equal(work.CreateTime.Time().UnixNano() / 1e6))
			Expect(work.Type).To(Equal(flowDetail.Workflow))
			Expect(work.State).To(Equal(flowDetail.StateMachine.States[0]))
			Expect(work.StateBeginTime).To(Equal(work.CreateTime))
			Expect(work.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))

			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: work.ID, SourceType: "WORK", SourceDesc: work.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			detail, err := workManager.WorkDetail(work.ID.String(), testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
			Expect(detail.Name).To(Equal(creation.Name))
			Expect(detail.Identifier).To(Equal(work.Identifier))
			Expect(detail.ProjectID).To(Equal(creation.ProjectID))
			Expect(detail.CreateTime.Time().Sub(time.Now()) < time.Minute).To(BeTrue())
			Expect(detail.Type).To(Equal(flowDetail.Workflow))
			Expect(detail.State).To(Equal(flowDetail.StateMachine.States[0]))
			Expect(detail.FlowID).To(Equal(flowDetail.ID))
			Expect(detail.OrderInState).To(Equal(work.CreateTime.Time().UnixNano() / 1e6))
			Expect(detail.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
			//Expect(len(work.Properties)).To(Equal(0))

			// should create init process step
			var initProcessStep []domain.WorkProcessStep
			Expect(testDatabase.DS.GormDB().Model(&domain.WorkProcessStep{}).Scan(&initProcessStep).Error).To(BeNil())
			Expect(initProcessStep).ToNot(BeNil())
			Expect(len(initProcessStep)).To(Equal(1))
			fmt.Println(initProcessStep[0].BeginTime, "detail:", detail.CreateTime, detail.StateBeginTime, "work:", work.CreateTime, work.StateBeginTime)
			Expect(initProcessStep[0]).To(Equal(domain.WorkProcessStep{WorkID: detail.ID, FlowID: detail.FlowID,
				CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
				StateName: detail.StateName, StateCategory: detail.State.Category, BeginTime: detail.CreateTime}))

			detail, err = workManager.WorkDetail(work.Identifier, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(detail.ID).To(Equal(work.ID))
		})

		It("should create new work with highest priority successfully", func() {
			creation := &domain.WorkCreation{Name: "test work", ProjectID: project2.ID, FlowID: flowDetail2.ID,
				InitialStateName: domain.StatePending.Name, PriorityLevel: -2}
			sec := testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String())
			ignoreWork1, err := workManager.CreateWork(creation, sec)
			Expect(err).To(BeZero())
			Expect(ignoreWork1).ToNot(BeZero())
			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: ignoreWork1.ID, SourceType: "WORK", SourceDesc: ignoreWork1.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			creation = &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID,
				InitialStateName: domain.StateDoing.Name}
			sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
			ignoreWork2, err := workManager.CreateWork(creation, sec)
			Expect(err).To(BeZero())
			Expect(ignoreWork2).ToNot(BeZero())
			Expect(ignoreWork2.OrderInState > ignoreWork1.OrderInState).To(BeTrue())
			Expect(len(lastEvents)).To(Equal(2))
			Expect(lastEvents[1].Event).To(Equal(event.Event{SourceId: ignoreWork2.ID, SourceType: "WORK", SourceDesc: ignoreWork2.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[1].Timestamp.Time()) < time.Second).To(BeTrue())

			sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
			creation = &domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID,
				InitialStateName: domain.StatePending.Name}
			work, err := workManager.CreateWork(creation, sec)
			Expect(err).To(BeZero())
			Expect(work).ToNot(BeZero())
			detail, err := workManager.WorkDetail(work.ID.String(), sec)
			Expect(err).To(BeNil())
			Expect(detail).ToNot(BeNil())
			Expect(work.OrderInState > ignoreWork2.OrderInState).To(BeTrue())
			Expect(len(lastEvents)).To(Equal(3))
			Expect(lastEvents[2].Event).To(Equal(event.Event{SourceId: work.ID, SourceType: "WORK", SourceDesc: work.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[2].Timestamp.Time()) < time.Second).To(BeTrue())

			creation = &domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID,
				InitialStateName: domain.StatePending.Name, PriorityLevel: -1}
			sec = testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String())
			work1, err := workManager.CreateWork(creation, sec)
			Expect(err).To(BeZero())
			Expect(work).ToNot(BeZero())
			Expect(len(lastEvents)).To(Equal(4))
			Expect(lastEvents[3].Event).To(Equal(event.Event{SourceId: work1.ID, SourceType: "WORK", SourceDesc: work1.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[3].Timestamp.Time()) < time.Second).To(BeTrue())
			detail1, err := workManager.WorkDetail(work1.ID.String(), sec)
			Expect(err).To(BeNil())
			Expect(detail1).ToNot(BeNil())

			Expect(detail1.StateName).To(Equal(domain.StatePending.Name))
			Expect(detail1.StateCategory).To(Equal(domain.StatePending.Category))
			Expect(detail1.OrderInState).To(Equal(detail.OrderInState - 1))
			Expect(detail1.OrderInState > ignoreWork2.OrderInState).To(BeTrue())
		})

		It("should forbid to create to other project", func() {
			creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(work).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
	})

	Describe("DetailWork", func() {
		It("should forbid to get work detail with permissions", func() {
			creation := &domain.WorkCreation{Name: "test work", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}
			work, err := workManager.CreateWork(creation, testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())

			detail, err := workManager.WorkDetail(work.ID.String(), testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(detail).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})
		It("should return error when work not found", func() {
			detail, err := workManager.WorkDetail(types.ID(404).String(), testinfra.BuildSecCtx(200, domain.ProjectRoleManager+"_"+project2.ID.String()))
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
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project2.ID, FlowID: flowDetail2.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String(), domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(0))

			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			work1 := (*works)[0]
			Expect(work1.ID).ToNot(BeZero())
			Expect(work1.Name).To(Equal("test work1"))
			Expect(work1.ProjectID).To(Equal(project1.ID))
			Expect(work1.CreateTime).ToNot(BeZero())
			Expect(work1.FlowID).To(Equal(flowDetail.ID))
			Expect(work1.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work1.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
		})

		It("should query by name and project id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project2.ID, FlowID: flowDetail2.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_"+project2.ID.String()))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(
				&domain.WorkQuery{Name: "work2", ProjectID: project1.ID},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			work1 := (*works)[0]
			Expect(work1.ID).ToNot(BeZero())
			Expect(work1.Name).To(Equal("test work2"))
			Expect(work1.ProjectID).To(Equal(project1.ID))
			Expect(work1.CreateTime).ToNot(BeZero())
			Expect(work1.FlowID).To(Equal(flowDetail.ID))
			Expect(work1.StateName).To(Equal(flowDetail.StateMachine.States[0].Name))
			Expect(work1.StateCategory).To(Equal(flowDetail.StateMachine.States[0].Category))
			Expect(work1.State).To(Equal(flowDetail.StateMachine.States[0]))
		})

		It("should query by stateCategory", func() {
			work1, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			work2, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			Expect(testDatabase.DS.GormDB().Model(&domain.Work{}).Where(&domain.Work{ID: work2.ID}).
				Update("state_category", state.InProcess).Error).To(BeNil())

			work3, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work3", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			Expect(testDatabase.DS.GormDB().Model(&domain.Work{}).Where(&domain.Work{ID: work3.ID}).
				Update("state_category", state.Done).Error).To(BeNil())

			works, err := workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID, StateCategories: []state.Category{state.InBacklog, state.InProcess}},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))
			Expect((*works)[0].ID).To(Equal(work1.ID))
			Expect((*works)[1].ID).To(Equal(work2.ID))
		})

		It("should be able to query by archive status", func() {
			secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			work1, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())
			work2, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StateDone.Name}, secCtx)
			Expect(err).To(BeZero())
			Expect(workManager.ArchiveWorks([]types.ID{work2.ID}, secCtx)).To(BeNil())
			work3, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work3", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StateDone.Name}, secCtx)
			Expect(err).To(BeZero())

			// default is OFF
			works, err := workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID}, secCtx)
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))
			Expect((*works)[0].ID).To(Equal(work1.ID))
			Expect((*works)[1].ID).To(Equal(work3.ID))

			works, err = workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID, ArchiveState: domain.StatusOff}, secCtx)
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))
			Expect((*works)[0].ID).To(Equal(work1.ID))
			Expect((*works)[1].ID).To(Equal(work3.ID))

			works, err = workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID, ArchiveState: domain.StatusOn}, secCtx)
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))
			Expect((*works)[0].ID).To(Equal(work2.ID))

			works, err = workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID, ArchiveState: domain.StatusAll}, secCtx)
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(3))
		})

		It("works should be ordered by orderInState asc and id asc", func() {
			now := types.CurrentTimestamp()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", Identifier: "W-1", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2, StateName: "PENDING", StateBeginTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 1, Name: "w2", Identifier: "W-2", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2, StateName: "PENDING", StateBeginTime: now}).Error).To(BeNil())
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 3, Name: "w3", Identifier: "W-3", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 1, StateName: "PENDING", StateBeginTime: now}).Error).To(BeNil())

			// order by orderInState:    w3(1) > w2(2) = w1(2)
			// order by id (default):         w2(1) > w1(2)
			works, err := workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(3))
			Expect((*works)[0].Name).To(Equal("w3"))
			Expect((*works)[1].Name).To(Equal("w2"))
			Expect((*works)[2].Name).To(Equal("w1"))
		})

		It("should return error if failed to find state", func() {
			now := types.CurrentTimestamp()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2, StateName: "UNKNOWN", StateBeginTime: now}).Error).To(BeNil())
			works, err := workManager.QueryWork(&domain.WorkQuery{ProjectID: project1.ID},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(works).To(BeNil())
			Expect(err).To(Equal(bizerror.ErrStateInvalid))
		})
	})

	Describe("UpdateWork", func() {
		It("should be able to update work", func() {
			sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				sec,
			)
			Expect(err).To(BeZero())
			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryCreated}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			updatedWork, err := workManager.UpdateWork(detail.ID,
				&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			Expect(updatedWork).ToNot(BeNil())
			Expect(updatedWork.ID).To(Equal(detail.ID))
			Expect(updatedWork.Name).To(Equal("test work1 new"))
			Expect(updatedWork.State).To(Equal(flowDetail.StateMachine.States[0]))

			Expect(len(lastEvents)).To(Equal(2))
			Expect(lastEvents[1].Event).To(Equal(event.Event{SourceId: detail.ID, SourceType: "WORK", SourceDesc: detail.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
					PropertyName: "Name", PropertyDesc: "Name", OldValue: detail.Name, OldValueDesc: detail.Name, NewValue: updatedWork.Name, NewValueDesc: updatedWork.Name,
				}}}))
			Expect(time.Now().Sub(lastEvents[1].Timestamp.Time()) < time.Second).To(BeTrue())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))

			Expect((*works)[0].ID).To(Equal(detail.ID))
			Expect((*works)[0].Name).To(Equal("test work1 new"))
			Expect((*works)[0].State).To(Equal(flowDetail.StateMachine.States[0]))
		})
		It("should be able to catch error when work not found", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			Expect(len(lastEvents)).To(Equal(1))

			updatedWork, err := workManager.UpdateWork(404,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("record not found")) // thrown when check permissions
			Expect(len(lastEvents)).To(Equal(1))
		})

		It("should forbid to update work without permission", func() {
			detail, err := workManager.CreateWork(&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			Expect(len(lastEvents)).To(Equal(1))

			updatedWork, err := workManager.UpdateWork(detail.ID,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_2"))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("forbidden"))
			Expect(len(lastEvents)).To(Equal(1))
		})

		It("should be able to catch db errors", func() {
			testDatabase.DS.GormDB().DropTable(&domain.Work{})

			updatedWork, err := workManager.UpdateWork(12345,
				&domain.WorkUpdating{Name: "test work1 new"}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(updatedWork).To(BeNil())
			Expect(err).ToNot(BeZero())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
			Expect(len(lastEvents)).To(Equal(0))
		})

		It("should return error if failed to find state", func() {
			now := types.CurrentTimestamp()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2, StateName: "UNKNOWN", StateBeginTime: now}).Error).To(BeNil())
			updatedWork, err := workManager.UpdateWork(2,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(updatedWork).To(BeNil())
			Expect(err).To(Equal(bizerror.ErrStateInvalid))
		})

		It("should failed when work is archived when update work", func() {
			now := types.CurrentTimestamp()
			Expect(testDatabase.DS.GormDB().Create(&domain.Work{ID: 2, Name: "w1", ProjectID: project1.ID,
				CreateTime: now, FlowID: flowDetail.ID, OrderInState: 2,
				StateName: domain.StateDone.Name, StateCategory: domain.StateDone.Category, StateBeginTime: now}).Error).To(BeNil())
			Expect(workManager.ArchiveWorks([]types.ID{2}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))).To(BeNil())

			updatedWork, err := workManager.UpdateWork(2,
				&domain.WorkUpdating{Name: "test work1 new"},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(updatedWork).To(BeNil())
			Expect(err).To(Equal(bizerror.ErrArchiveStatusInvalid))
		})
	})

	Describe("DeleteWork", func() {
		It("should be able to delete work by id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(
				&domain.WorkCreation{Name: "test work2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(2))

			lastEvents = []event.EventRecord{}
			// do delete work
			workToDelete := (*works)[0]
			sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			err = workManager.DeleteWork(workToDelete.ID, sec)
			Expect(err).To(BeNil())
			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: workToDelete.ID, SourceType: "WORK", SourceDesc: workToDelete.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryDeleted}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			works, err = workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(1))

			// work process steps should also be deleted
			processStep := domain.WorkProcessStep{}
			Expect(testDatabase.DS.GormDB().First(&processStep, domain.WorkProcessStep{WorkID: workToDelete.ID}).Error).To(Equal(gorm.ErrRecordNotFound))
			processStep = domain.WorkProcessStep{}
			Expect(testDatabase.DS.GormDB().First(&processStep).Error).To(BeNil())
		})

		It("should forbid to delete without permissions", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_123"))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should be able to catch db errors", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			Expect(testDatabase.DS.GormDB().DropTable(&domain.WorkProcessStep{}).Error).To(BeNil())
			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".work_process_steps' doesn't exist"))

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			err = workManager.DeleteWork(detail.ID, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})
	})

	Describe("ArchiveWorks", func() {
		It("should forbid to archive without permissions", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			err = workManager.ArchiveWorks([]types.ID{detail.ID}, testinfra.BuildSecCtx(2, domain.ProjectRoleManager+"_123"))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("forbidden"))
		})

		It("should not be able to archive when work is not in a completed state", func() {
			secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				secCtx)
			Expect(err).To(BeZero())

			err = workManager.ArchiveWorks([]types.ID{detail.ID}, secCtx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(bizerror.ErrStateCategoryInvalid))
		})

		It("should be able to catch db errors when archive work", func() {
			detail, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			testDatabase.DS.GormDB().DropTable(&domain.Work{})
			err = workManager.ArchiveWorks([]types.ID{detail.ID}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Error 1146: Table '" + testDatabase.TestDatabaseName + ".works' doesn't exist"))
		})

		It("should be able to archive work by id", func() {
			_, err := workManager.CreateWork(
				&domain.WorkCreation{Name: "test work1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StateDone.Name},
				testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeZero())

			works, err := workManager.QueryWork(&domain.WorkQuery{}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect(works).ToNot(BeNil())
			Expect(len(*works)).To(Equal(1))
			Expect((*works)[0].ArchiveTime.IsZero()).To(BeTrue())

			// do archive work
			lastEvents = []event.EventRecord{}
			sec := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			workToArchive := (*works)[0]
			err = workManager.ArchiveWorks([]types.ID{workToArchive.ID}, sec)
			Expect(err).To(BeNil())
			works, err = workManager.QueryWork(&domain.WorkQuery{ArchiveState: domain.StatusOn}, sec)
			Expect(err).To(BeNil())
			Expect(len(*works)).To(Equal(1))
			archivedWork := (*works)[0]
			Expect(archivedWork.ArchiveTime).ToNot(BeNil())

			Expect(len(lastEvents)).To(Equal(1))
			Expect(lastEvents[0].Event).To(Equal(event.Event{SourceId: workToArchive.ID, SourceType: "WORK", SourceDesc: workToArchive.Identifier,
				CreatorId: sec.Identity.ID, CreatorName: sec.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
					PropertyName: "ArchiveTime", PropertyDesc: "ArchiveTime",
					OldValue: workToArchive.ArchiveTime.String(), OldValueDesc: workToArchive.ArchiveTime.String(),
					NewValue: archivedWork.ArchiveTime.String(), NewValueDesc: archivedWork.ArchiveTime.String(),
				}}}))
			Expect(time.Now().Sub(lastEvents[0].Timestamp.Time()) < time.Second).To(BeTrue())

			// do archive again
			err = workManager.ArchiveWorks([]types.ID{workToArchive.ID}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			works1, err := workManager.QueryWork(&domain.WorkQuery{ArchiveState: domain.StatusAll}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String()))
			Expect(err).To(BeNil())
			Expect((*works1)[0].ArchiveTime).To(Equal((*works)[0].ArchiveTime))
			Expect(len(lastEvents)).To(Equal(1))
		})
	})

	Describe("UpdateStateRangeOrders", func() {
		It("should do nothing when input is empty", func() {
			Expect(workManager.UpdateStateRangeOrders(nil, nil)).To(BeNil())
			Expect(workManager.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{}, nil)).To(BeNil())
		})

		It("should be able to handle forbidden access", func() {
			Expect(workManager.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
				{ID: 1, NewOlder: 3, OldOlder: 2}}, nil)).To(Equal(errors.New("record not found")))
			Expect(workManager.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
				{ID: 1, NewOlder: 3, OldOlder: 2}}, testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_404"))).To(Equal(errors.New("record not found")))
		})

		It("should update order", func() {
			secCtx := testinfra.BuildSecCtx(1, domain.ProjectRoleManager+"_"+project1.ID.String())
			_, err := workManager.CreateWork(&domain.WorkCreation{Name: "w1", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "w2", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())
			_, err = workManager.CreateWork(&domain.WorkCreation{Name: "w3", ProjectID: project1.ID, FlowID: flowDetail.ID, InitialStateName: domain.StatePending.Name}, secCtx)
			Expect(err).To(BeZero())

			// default w1 > w2 > w3
			listPtr, err := workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list := *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

			// invalid data
			Expect(workManager.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{{ID: list[0].ID, NewOlder: 3, OldOlder: 2}}, secCtx)).
				To(Equal(errors.New("expected affected row is 1, but actual is 0")))

			listPtr, err = workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list = *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w1", "w2", "w3"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())

			lastEvents = []event.EventRecord{}
			// valid data: w3 > w2 > w1
			Expect(workManager.UpdateStateRangeOrders(&[]domain.WorkOrderRangeUpdating{
				{ID: list[0].ID, NewOlder: list[2].OrderInState + 2, OldOlder: list[0].OrderInState},
				{ID: list[1].ID, NewOlder: list[2].OrderInState + 1, OldOlder: list[1].OrderInState}}, secCtx)).To(BeNil())

			Expect(len(lastEvents)).To(Equal(2))
			lastStateOrderUpdatedWork := list[1]
			Expect(lastEvents[1].Event).To(Equal(event.Event{SourceId: lastStateOrderUpdatedWork.ID, SourceType: "WORK", SourceDesc: lastStateOrderUpdatedWork.Identifier,
				CreatorId: secCtx.Identity.ID, CreatorName: secCtx.Identity.Name, EventCategory: event.EventCategoryPropertyUpdated, UpdatedProperties: []event.UpdatedProperty{{
					PropertyName: "OrderInState", PropertyDesc: "OrderInState",
					OldValue: strconv.FormatInt(lastStateOrderUpdatedWork.OrderInState, 10), OldValueDesc: strconv.FormatInt(lastStateOrderUpdatedWork.OrderInState, 10),
					NewValue: strconv.FormatInt(list[2].OrderInState+1, 10), NewValueDesc: strconv.FormatInt(list[2].OrderInState+1, 10),
				}}}))
			Expect(time.Now().Sub(lastEvents[1].Timestamp.Time()) < time.Second).To(BeTrue())

			listPtr, err = workManager.QueryWork(&domain.WorkQuery{}, secCtx)
			Expect(err).To(BeNil())
			list = *listPtr
			Expect(len(list)).To(Equal(3))
			Expect([]string{list[0].Name, list[1].Name, list[2].Name}).To(Equal([]string{"w3", "w2", "w1"}))
			Expect(list[0].OrderInState < list[1].OrderInState && list[1].OrderInState < list[2].OrderInState).To(BeTrue())
		})
	})
})
