package work

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/persistence"
	"flywheel/security"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"strconv"
	"time"
)

type WorkManagerTraits interface {
	QueryWork(query *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error)
	WorkDetail(id types.ID, sec *security.Context) (*domain.WorkDetail, error)
	CreateWork(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error)
	UpdateWork(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error)
	DeleteWork(id types.ID, sec *security.Context) error
	UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error

	ArchiveWorks(ids []types.ID, sec *security.Context) error
}

type WorkManager struct {
	workflowManager flow.WorkflowManagerTraits
	dataSource      *persistence.DataSourceManager
	idWorker        *sonyflake.Sonyflake
}

func NewWorkManager(ds *persistence.DataSourceManager, workflowManager flow.WorkflowManagerTraits) *WorkManager {
	return &WorkManager{
		workflowManager: workflowManager,
		dataSource:      ds,
		idWorker:        sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *WorkManager) QueryWork(query *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
	var works []domain.Work
	db := m.dataSource.GormDB()

	q := db.Where(domain.Work{GroupID: query.GroupID})
	if query.Name != "" {
		q = q.Where("name like ?", "%"+query.Name+"%")
	}
	if len(query.StateCategories) > 0 {
		q = q.Where("state_category in (?)", query.StateCategories)
	}
	visibleGroups := sec.VisibleGroups()
	if len(visibleGroups) == 0 {
		return &[]domain.Work{}, nil
	}
	q = q.Where("group_id in (?)", visibleGroups).Order("order_in_state ASC")
	if err := q.Find(&works).Error; err != nil {
		return nil, err
	}

	// append Work.state
	workflowCache := map[types.ID]*domain.WorkflowDetail{}
	var err error
	for i := len(works) - 1; i >= 0; i-- {
		work := works[i]
		workflow := workflowCache[work.FlowID]
		if workflow == nil {
			workflow, err = m.workflowManager.DetailWorkflow(work.FlowID, sec)
			if err != nil {
				return nil, err
			}
			workflowCache[work.FlowID] = workflow
		}

		stateFound, found := workflow.FindState(work.StateName)
		if !found {
			return nil, domain.ErrInvalidState
		}
		works[i].State = stateFound
	}

	return &works, nil
}

func (m *WorkManager) WorkDetail(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
	workDetail := domain.WorkDetail{}
	db := m.dataSource.GormDB()
	if err := db.Where(&domain.Work{ID: id}).First(&(workDetail.Work)).Error; err != nil {
		return nil, err
	}

	if !sec.HasRoleSuffix("_" + workDetail.GroupID.String()) {
		return nil, bizerror.ErrForbidden
	}

	workflowDetail, err := m.workflowManager.DetailWorkflow(workDetail.FlowID, sec)
	if err != nil {
		return nil, err
	}
	workDetail.Type = workflowDetail.Workflow
	stateFound, found := workflowDetail.FindState(workDetail.StateName)
	if !found {
		return nil, domain.ErrInvalidState
	}
	workDetail.State = stateFound

	return &workDetail, nil
}

func (m *WorkManager) CreateWork(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
	if !sec.HasRoleSuffix("_" + c.GroupID.String()) {
		return nil, bizerror.ErrForbidden
	}

	db := m.dataSource.GormDB()
	var workDetail *domain.WorkDetail
	err := db.Transaction(func(tx *gorm.DB) error {
		// TODO transition issues
		workflowDetail, err := m.workflowManager.DetailWorkflow(c.FlowID, sec)
		if err != nil {
			return err
		}

		initialState, found := workflowDetail.StateMachine.FindState(c.InitialStateName)
		if !found {
			return bizerror.ErrUnknownState
		}

		workDetail = BuildWorkDetail(common.NextId(m.idWorker), c, workflowDetail, initialState)
		if c.PriorityLevel < 0 {
			var highestPriorityWork domain.Work
			err := tx.Model(&domain.Work{}).Where(&domain.Work{GroupID: c.GroupID, StateName: initialState.Name}).
				Select("order_in_state").
				Order("order_in_state ASC").Limit(1).First(&highestPriorityWork).Error
			if err == nil {
				workDetail.OrderInState = highestPriorityWork.OrderInState - 1
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Create(workDetail.Work).Error; err != nil {
			return err
		}

		initProcessStep := domain.WorkProcessStep{WorkID: workDetail.ID, FlowID: workDetail.FlowID,
			StateName: workDetail.State.Name, StateCategory: workDetail.State.Category, BeginTime: workDetail.CreateTime}
		if err := tx.Create(initProcessStep).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return workDetail, nil
}

func (m *WorkManager) UpdateWork(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
	var work domain.Work
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		if err := m.checkPerms(tx, id, sec); err != nil {
			return err
		}

		db := tx.Model(&domain.Work{}).Where(&domain.Work{ID: id}).Update(u)
		if err := db.Error; err != nil {
			return err
		}
		if db.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(db.RowsAffected, 10))
		}
		if err := tx.Where(&domain.Work{ID: id}).First(&work).Error; err != nil {
			return err
		}
		// TODO transition issues
		workflowDetail, err := m.workflowManager.DetailWorkflow(work.FlowID, sec)
		if err != nil {
			return err
		}
		stateFound, found := workflowDetail.FindState(work.StateName)
		if !found {
			return domain.ErrInvalidState
		}
		work.State = stateFound

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &work, nil
}

func (m *WorkManager) UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *security.Context) error {
	if wantedOrders == nil || len(*wantedOrders) == 0 {
		return nil
	}

	return m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		for _, orderUpdating := range *wantedOrders {
			// TODO transition issues
			if err := m.checkPerms(tx, orderUpdating.ID, sec); err != nil {
				return err
			}
			db := tx.Model(&domain.Work{}).Where(&domain.Work{ID: orderUpdating.ID, OrderInState: orderUpdating.OldOlder}).
				Update(&domain.Work{OrderInState: orderUpdating.NewOlder})
			if err := db.Error; err != nil {
				return err
			}
			if db.RowsAffected != 1 {
				return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(db.RowsAffected, 10))
			}
		}
		return nil
	})
}

func (m *WorkManager) DeleteWork(id types.ID, sec *security.Context) error {
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		if err := m.checkPerms(tx, id, sec); err != nil {
			return err
		}
		if err := tx.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(domain.WorkStateTransition{}, "work_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(domain.WorkProcessStep{}, "work_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})

	return err
}

func (m *WorkManager) ArchiveWorks(ids []types.ID, sec *security.Context) error {
	now := time.Now()
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if err := m.checkPerms(tx, id, sec); err != nil {
				return err
			}

			var work domain.Work
			if err := m.dataSource.GormDB().Where(&domain.Work{ID: id}).First(&work).Error; err != nil {
				return err
			}
			if work.StateCategory != state.Done && work.StateCategory != state.Rejected {
				return bizerror.ErrStateInvalid
			}

			db := tx.Model(&domain.Work{ID: id}).Updates(&domain.Work{ArchiveTime: &now})
			if err := db.Error; err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (m *WorkManager) checkPerms(db *gorm.DB, id types.ID, sec *security.Context) error {
	var work domain.Work
	if err := db.Where(&domain.Work{ID: id}).First(&work).Error; err != nil {
		return err
	}
	if sec == nil || !sec.HasRoleSuffix("_"+work.GroupID.String()) {
		return bizerror.ErrForbidden
	}
	return nil
}

func BuildWorkDetail(id types.ID, c *domain.WorkCreation, workflow *domain.WorkflowDetail, initState state.State) *domain.WorkDetail {
	now := time.Now()
	return &domain.WorkDetail{
		Work: domain.Work{
			ID:         id,
			Name:       c.Name,
			GroupID:    c.GroupID,
			CreateTime: now,

			FlowID:         workflow.ID,
			OrderInState:   now.UnixNano() / 1e6,
			StateName:      initState.Name,
			StateCategory:  initState.Category,
			StateBeginTime: &now,
			State:          initState,
		},
		Type: workflow.Workflow,
	}
}
