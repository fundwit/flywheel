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
)

var (
	QueryProcessStepsFunc         = QueryProcessSteps
	CreateWorkStateTransitionFunc = CreateWorkStateTransition
)

func QueryProcessSteps(query *domain.WorkProcessStepQuery, s *session.Session) (*[]domain.WorkProcessStep, error) {
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	work := domain.Work{}
	if err := db.Where(&domain.Work{ID: query.WorkID}).Select("project_id").First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &[]domain.WorkProcessStep{}, nil
		} else {
			return nil, err
		}
	}
	if !s.Perms.HasProjectViewPerm(work.ProjectID) {
		return &[]domain.WorkProcessStep{}, nil
	}

	var processSteps []domain.WorkProcessStep
	if err := db.Where(&domain.WorkProcessStep{WorkID: query.WorkID}).Find(&processSteps).Error; err != nil {
		return nil, err
	}
	return &processSteps, nil
}

func CreateWorkStateTransition(c *domain.WorkProcessStepCreation, s *session.Session) error {
	workflow, err := flow.DetailWorkflowFunc(c.FlowID, s)
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

	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	var ev *event.EventRecord
	err = db.Transaction(func(tx *gorm.DB) error {
		// check perms
		work := domain.Work{ID: c.WorkID}
		if err := tx.Where(&work).First(&work).Error; err != nil {
			return err
		}
		if !s.Perms.HasRoleSuffix("_" + work.ProjectID.String()) {
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
		ret := tx.Model(&domain.WorkProcessStep{}).
			Where(&domain.WorkProcessStep{WorkID: c.WorkID, FlowID: workflow.ID, StateName: fromState.Name}).
			Where("end_time = ?", types.Timestamp{}).
			Update(&domain.WorkProcessStep{EndTime: now, NextStateName: toState.Name, NextStateCategory: toState.Category})
		if ret.Error != nil {
			return err
		}
		if ret.RowsAffected != 1 {
			return bizerror.ErrWorkProcessStepStateInvalid
		}
		nextProcessStep := domain.WorkProcessStep{WorkID: work.ID, FlowID: work.FlowID, CreatorID: s.Identity.ID, CreatorName: s.Identity.Nickname,
			StateName: toState.Name, StateCategory: toState.Category, BeginTime: now}
		if err := tx.Create(nextProcessStep).Error; err != nil {
			return err
		}

		ev, err = CreateWorkPropertyUpdatedEvent(&work,
			[]event.UpdatedProperty{{
				PropertyName: "StateName", PropertyDesc: "StateName", OldValue: work.StateName, OldValueDesc: work.StateName, NewValue: c.ToState, NewValueDesc: c.ToState,
			}},
			&s.Identity, now, tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}

	return nil
}
