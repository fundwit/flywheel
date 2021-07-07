package work

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/event"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"
	"strconv"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

// workflows
// workflow_states
// workflow_state_transitions

/*
{
	"id": ...
	"name": ...
	"projectId": ...
	"createTime": ...
	"themeColor": ...
	"themeIcon": ...
	"states": [{
		// workflowId, createTime
		"name": ...
		"category": ...
		"order": ...
	}],
	"transitions": [{
		// worklfowId, createTime, name
		"fromState": ...
		"toState": ...
	}],
	"properties": [{
		...
	}]
}
*/

// projects
// project_members

// users
// user_role_bindings
// roles
// role_permission_bindings
// permissions

type WorkManagerTraits interface {
	QueryWork(query *domain.WorkQuery, sec *session.Context) (*[]domain.Work, error)
	WorkDetail(id string, sec *session.Context) (*domain.WorkDetail, error)
	CreateWork(c *domain.WorkCreation, sec *session.Context) (*domain.WorkDetail, error)
	UpdateWork(id types.ID, u *domain.WorkUpdating, sec *session.Context) (*domain.Work, error)
	DeleteWork(id types.ID, sec *session.Context) error
	UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *session.Context) error

	ArchiveWorks(ids []types.ID, sec *session.Context) error
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

func (m *WorkManager) QueryWork(query *domain.WorkQuery, sec *session.Context) (*[]domain.Work, error) {
	var works []domain.Work
	db := m.dataSource.GormDB()

	q := db.Where(domain.Work{ProjectID: query.ProjectID})
	if query.Name != "" {
		q = q.Where("name like ?", "%"+query.Name+"%")
	}
	if len(query.StateCategories) > 0 {
		q = q.Where("state_category in (?)", query.StateCategories)
	}

	if query.ArchiveState == domain.StatusOn {
		q = q.Where("archive_time != ?", types.Timestamp{})
	} else if query.ArchiveState == domain.StatusAll {
		// archive_time not in where clause
	} else {
		q = q.Where("archive_time = ?", types.Timestamp{})
	}

	visibleProjects := sec.VisibleProjects()
	if len(visibleProjects) == 0 {
		return &[]domain.Work{}, nil
	}
	q = q.Where("project_id in (?)", visibleProjects).Order("order_in_state ASC")
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
			return nil, bizerror.ErrStateInvalid
		}
		works[i].State = stateFound
	}

	return &works, nil
}

func (m *WorkManager) WorkDetail(identifier string, sec *session.Context) (*domain.WorkDetail, error) {
	id, _ := types.ParseID(identifier)
	workDetail := domain.WorkDetail{}
	db := m.dataSource.GormDB()
	if err := db.Where("id = ? OR identifier LIKE ?", id, identifier).First(&(workDetail.Work)).Error; err != nil {
		return nil, err
	}

	if !sec.HasRoleSuffix("_" + workDetail.ProjectID.String()) {
		return nil, bizerror.ErrForbidden
	}

	workflowDetail, err := m.workflowManager.DetailWorkflow(workDetail.FlowID, sec)
	if err != nil {
		return nil, err
	}
	workDetail.Type = workflowDetail.Workflow
	stateFound, found := workflowDetail.FindState(workDetail.StateName)
	if !found {
		return nil, bizerror.ErrStateInvalid
	}
	workDetail.State = stateFound

	return &workDetail, nil
}

func (m *WorkManager) CreateWork(c *domain.WorkCreation, sec *session.Context) (*domain.WorkDetail, error) {
	if !sec.HasRoleSuffix("_" + c.ProjectID.String()) {
		return nil, bizerror.ErrForbidden
	}

	db := m.dataSource.GormDB()
	var workDetail *domain.WorkDetail
	err := db.Transaction(func(tx *gorm.DB) error {
		workflowDetail, err := m.workflowManager.DetailWorkflow(c.FlowID, sec)
		if err != nil {
			return err
		}
		initialState, found := workflowDetail.StateMachine.FindState(c.InitialStateName)
		if !found {
			return bizerror.ErrUnknownState
		}

		now := types.CurrentTimestamp()
		workDetail = &domain.WorkDetail{
			Work: domain.Work{
				ID:         idgen.NextID(m.idWorker),
				Name:       c.Name,
				ProjectID:  c.ProjectID,
				CreateTime: now,

				FlowID:         workflowDetail.ID,
				OrderInState:   now.Time().UnixNano() / 1e6, // oldest
				StateName:      initialState.Name,
				StateCategory:  initialState.Category,
				StateBeginTime: now,
				State:          initialState,
			},
			Type: workflowDetail.Workflow,
		}
		if c.PriorityLevel < 0 { // Highest: -1, lowest： 1
			var highestPriorityWork domain.Work
			err := tx.Model(&domain.Work{}).Where(&domain.Work{ProjectID: c.ProjectID, StateName: initialState.Name}).
				Select("order_in_state").
				Order("order_in_state ASC").Limit(1).First(&highestPriorityWork).Error
			if err == nil {
				workDetail.OrderInState = highestPriorityWork.OrderInState - 1
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		identifier, err := namespace.NextWorkIdentifier(c.ProjectID, tx)
		if err != nil {
			return err
		}
		workDetail.Identifier = identifier

		if err := tx.Create(workDetail.Work).Error; err != nil {
			return err
		}

		initProcessStep := domain.WorkProcessStep{WorkID: workDetail.ID, FlowID: workDetail.FlowID,
			CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
			StateName: workDetail.State.Name, StateCategory: workDetail.State.Category, BeginTime: workDetail.CreateTime}
		if err := tx.Create(initProcessStep).Error; err != nil {
			return err
		}

		if err := CreateWorkCreatedEvent(&workDetail.Work, &sec.Identity, tx); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return workDetail, nil
}

func (m *WorkManager) UpdateWork(id types.ID, u *domain.WorkUpdating, sec *session.Context) (*domain.Work, error) {
	var updatedWork domain.Work
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		originWork, err := m.findWorkAndCheckPerms(tx, id, sec)
		if err != nil {
			return err
		}

		if !originWork.ArchiveTime.IsZero() {
			return bizerror.ErrArchiveStatusInvalid
		}

		db := tx.Model(&domain.Work{}).Where(&domain.Work{ID: id}).Update(u)
		if err := db.Error; err != nil {
			return err
		}
		if db.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(db.RowsAffected, 10))
		}

		if err := CreateWorkPropertyUpdatedEvent(originWork,
			[]event.UpdatedProperty{{
				PropertyName: "Name", PropertyDesc: "Name",
				OldValue: originWork.Name, OldValueDesc: originWork.Name,
				NewValue: u.Name, NewValueDesc: u.Name,
			}},
			&sec.Identity, tx); err != nil {
			return err
		}

		// append detail
		if err := tx.Where(&domain.Work{ID: id}).First(&updatedWork).Error; err != nil {
			return err
		}
		workflowDetail, err := m.workflowManager.DetailWorkflow(updatedWork.FlowID, sec)
		if err != nil {
			return err
		}
		stateFound, found := workflowDetail.FindState(updatedWork.StateName)
		if !found {
			return bizerror.ErrStateInvalid
		}
		updatedWork.State = stateFound

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &updatedWork, nil
}

func (m *WorkManager) UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, sec *session.Context) error {
	if wantedOrders == nil || len(*wantedOrders) == 0 {
		return nil
	}

	return m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		for _, orderUpdating := range *wantedOrders {
			// TODO transition issues
			originWork, err := m.findWorkAndCheckPerms(tx, orderUpdating.ID, sec)
			if err != nil {
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
			if err := CreateWorkPropertyUpdatedEvent(originWork,
				[]event.UpdatedProperty{{
					PropertyName: "OrderInState", PropertyDesc: "OrderInState",
					OldValue: strconv.FormatInt(originWork.OrderInState, 10), OldValueDesc: strconv.FormatInt(originWork.OrderInState, 10),
					NewValue: strconv.FormatInt(orderUpdating.NewOlder, 10), NewValueDesc: strconv.FormatInt(orderUpdating.NewOlder, 10),
				}},
				&sec.Identity, tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *WorkManager) DeleteWork(id types.ID, sec *session.Context) error {
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		_, err := m.findWorkAndCheckPerms(tx, id, sec)
		if err != nil {
			return err
		}
		work := domain.Work{ID: id}
		err = tx.Model(&work).First(&work).Error
		if err == nil {
			if err := CreateWorkDeletedEvent(&work, &sec.Identity, tx); err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(domain.WorkProcessStep{}, "work_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})

	return err
}

func (m *WorkManager) ArchiveWorks(ids []types.ID, sec *session.Context) error {
	now := types.CurrentTimestamp()
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			work, err := m.findWorkAndCheckPerms(tx, id, sec)
			if err != nil {
				return err
			}
			if work.StateCategory != state.Done && work.StateCategory != state.Rejected {
				return bizerror.ErrStateCategoryInvalid
			}
			if !work.ArchiveTime.IsZero() {
				return nil
			}

			if err := CreateWorkPropertyUpdatedEvent(work,
				[]event.UpdatedProperty{{
					PropertyName: "ArchiveTime", PropertyDesc: "ArchiveTime",
					OldValue: work.ArchiveTime.String(), OldValueDesc: work.ArchiveTime.String(),
					NewValue: now.String(), NewValueDesc: now.String(),
				}},
				&sec.Identity, tx); err != nil {
				return err
			}

			db := tx.Model(&domain.Work{ID: id}).Updates(&domain.Work{ArchiveTime: now})
			if err := db.Error; err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (m *WorkManager) findWorkAndCheckPerms(db *gorm.DB, id types.ID, sec *session.Context) (*domain.Work, error) {
	var work domain.Work
	if err := db.Where(&domain.Work{ID: id}).First(&work).Error; err != nil {
		return nil, err
	}
	if sec == nil || !sec.HasRoleSuffix("_"+work.ProjectID.String()) {
		return nil, bizerror.ErrForbidden
	}
	return &work, nil
}
