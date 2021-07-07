package work

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/event"
	"flywheel/persistence"
	"flywheel/session"
	"strconv"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

type WorkProcessManagerTraits interface {
	QueryProcessSteps(query *domain.WorkProcessStepQuery, sec *session.Context) (*[]domain.WorkProcessStep, error)
	CreateWorkStateTransition(*domain.WorkProcessStepCreation, *session.Context) error
}

type WorkProcessManager struct {
	dataSource      *persistence.DataSourceManager
	workflowManager flow.WorkflowManagerTraits
	idWorker        *sonyflake.Sonyflake
}

func NewWorkProcessManager(ds *persistence.DataSourceManager, workflowManger flow.WorkflowManagerTraits) *WorkProcessManager {
	return &WorkProcessManager{
		dataSource:      ds,
		workflowManager: workflowManger,
		idWorker:        sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *WorkProcessManager) QueryProcessSteps(query *domain.WorkProcessStepQuery, sec *session.Context) (*[]domain.WorkProcessStep, error) {
	db := m.dataSource.GormDB()
	work := domain.Work{}
	if err := db.Where(&domain.Work{ID: query.WorkID}).Select("project_id").First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &[]domain.WorkProcessStep{}, nil
		} else {
			return nil, err
		}
	}
	if !sec.HasRoleSuffix("_" + work.ProjectID.String()) {
		return &[]domain.WorkProcessStep{}, nil
	}

	var processSteps []domain.WorkProcessStep
	if err := db.Where(&domain.WorkProcessStep{WorkID: query.WorkID}).Find(&processSteps).Error; err != nil {
		return nil, err
	}
	return &processSteps, nil
}

func (m *WorkProcessManager) CreateWorkStateTransition(c *domain.WorkProcessStepCreation, sec *session.Context) error {
	workflow, err := m.workflowManager.DetailWorkflow(c.FlowID, sec)
	if err != nil {
		return err
	}
	// check whether the transition is acceptable
	availableTransitions := workflow.StateMachine.AvailableTransitions(c.FromState, c.ToState)
	if len(availableTransitions) != 1 {
		return errors.New("transition from " + c.FromState + " to " + c.ToState + " is not invalid")
	}

	now := types.CurrentTimestamp()
	fromState, found := workflow.FindState(c.FromState)
	if !found {
		return errors.New("invalid state " + fromState.Name)
	}
	toState, found := workflow.FindState(c.ToState)
	if !found {
		return errors.New("invalid state " + toState.Name)
	}

	db := m.dataSource.GormDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// check perms
		work := domain.Work{ID: c.WorkID}
		if err := tx.Where(&work).First(&work).Error; err != nil {
			return err
		}
		if !sec.HasRoleSuffix("_" + work.ProjectID.String()) {
			return bizerror.ErrForbidden
		}
		if !work.ArchiveTime.IsZero() {
			return bizerror.ErrArchiveStatusInvalid
		}

		query := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID, StateName: c.FromState}).
			Update(&domain.Work{StateName: c.ToState, StateCategory: toState.Category, StateBeginTime: now})
		if err := query.Error; err != nil {
			return err
		}
		if query.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(query.RowsAffected, 10))
		}
		if err := CreateWorkPropertyUpdatedEvent(&work,
			[]event.UpdatedProperty{{
				PropertyName: "StateName", PropertyDesc: "StateName", OldValue: work.StateName, OldValueDesc: work.StateName, NewValue: c.ToState, NewValueDesc: c.ToState,
			}},
			&sec.Identity, tx); err != nil {
			return err
		}

		// update work: beginProcessTime and endProcessTime
		if work.ProcessBeginTime.IsZero() && toState.Category != state.InBacklog {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_begin_time", &now).Error; err != nil {
				return err
			}
		}
		if work.ProcessEndTime.IsZero() && toState.Category == state.Done {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_end_time", &now).Error; err != nil {
				return err
			}
		} else if !work.ProcessEndTime.IsZero() && toState.Category != state.Done {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_end_time", nil).Error; err != nil {
				return err
			}
		}

		// update process step
		db := tx.Model(&domain.WorkProcessStep{}).
			Where(&domain.WorkProcessStep{WorkID: c.WorkID, FlowID: workflow.ID, StateName: fromState.Name}).
			Where("end_time = ?", types.Timestamp{}).
			Update(&domain.WorkProcessStep{EndTime: now, NextStateName: toState.Name, NextStateCategory: toState.Category})
		if db.Error != nil {
			return err
		}
		if db.RowsAffected != 1 {
			return bizerror.ErrWorkProcessStepStateInvalid
		}
		nextProcessStep := domain.WorkProcessStep{WorkID: work.ID, FlowID: work.FlowID, CreatorID: sec.Identity.ID, CreatorName: sec.Identity.Nickname,
			StateName: toState.Name, StateCategory: toState.Category, BeginTime: now}
		if err := tx.Create(nextProcessStep).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
