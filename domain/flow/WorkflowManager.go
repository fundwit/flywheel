package flow

import (
	"errors"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/state"
	"flywheel/persistence"
	"flywheel/security"
	"fmt"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"strconv"
	"time"
)

type WorkflowManagerTraits interface {
	QueryWorkflows(sec *security.Context) (*[]domain.WorkflowDetail, error)
	CreateWorkflow(c *WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error)
	DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error)

	CreateWorkStateTransition(*WorkStateTransitionBrief, *security.Context) (*WorkStateTransition, error)
}

type WorkflowManager struct {
	dataSource *persistence.DataSourceManager
	idWorker   *sonyflake.Sonyflake
}

type WorkflowCreation struct {
	Name         string             `json:"name"     binding:"required"`
	GroupID      types.ID           `json:"groupId" binding:"required"`
	StateMachine state.StateMachine `json:"stateMachine" binding:"required"`
}

func NewWorkflowManager(ds *persistence.DataSourceManager) WorkflowManagerTraits {
	return &WorkflowManager{
		dataSource: ds,
		idWorker:   sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *WorkflowManager) CreateWorkflow(c *WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error) {
	if !sec.HasRoleSuffix("_" + c.GroupID.String()) {
		return nil, common.ErrForbidden
	}

	workflow := &domain.WorkflowDetail{
		Workflow: domain.Workflow{
			ID:         common.NextId(m.idWorker),
			Name:       c.Name,
			GroupID:    c.GroupID,
			CreateTime: time.Now(),
		},
		StateMachine: c.StateMachine,
	}

	db := m.dataSource.GormDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(workflow.Workflow).Error; err != nil {
			return err
		}
		for idx, s := range workflow.StateMachine.States {
			stateEntity := &domain.WorkflowState{
				WorkflowID: workflow.ID, Order: idx, Name: s.Name, Category: s.Category, CreateTime: workflow.CreateTime,
			}
			if err := tx.Create(stateEntity).Error; err != nil {
				return err
			}
		}
		for _, t := range workflow.StateMachine.Transitions {
			transition := &domain.WorkflowStateTransition{
				WorkflowID: workflow.ID, Name: t.Name, FromState: t.From.Name, ToState: t.To.Name, CreateTime: workflow.CreateTime,
			}
			if err := tx.Create(transition).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (m *WorkflowManager) DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
	if ID == domain.GenericWorkFlow.ID {
		return &domain.GenericWorkFlow, nil
	}
	return nil, domain.ErrNotFound
}

func (m *WorkflowManager) QueryWorkflows(sec *security.Context) (*[]domain.WorkflowDetail, error) {
	return &[]domain.WorkflowDetail{domain.GenericWorkFlow}, nil
}

func (m *WorkflowManager) CreateWorkStateTransition(c *WorkStateTransitionBrief, sec *security.Context) (*WorkStateTransition, error) {
	flow, err := m.DetailWorkflow(c.FlowID, sec)
	if flow == nil {
		return nil, errors.New("workflow " + strconv.FormatUint(uint64(c.FlowID), 10) + " not found")
	}
	// check whether the transition is acceptable
	availableTransitions := flow.StateMachine.AvailableTransitions(c.FromState, c.ToState)
	if len(availableTransitions) != 1 {
		return nil, errors.New("transition from " + c.FromState + " to " + c.ToState + " is not invalid")
	}

	now := time.Now()
	newId := common.NextId(m.idWorker)
	transition := &WorkStateTransition{ID: newId, CreateTime: now, Creator: sec.Identity.ID, WorkStateTransitionBrief: *c}

	fromState, found := flow.FindState(c.FromState)
	if !found {
		return nil, errors.New("invalid state " + fromState.Name)
	}
	toState, found := flow.FindState(c.ToState)
	if !found {
		return nil, errors.New("invalid state " + toState.Name)
	}

	db := m.dataSource.GormDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// check perms
		work := domain.Work{ID: c.WorkID}
		if err := tx.Where(&work).First(&work).Error; err != nil {
			return err
		}
		if !sec.HasRole(fmt.Sprintf("%s_%d", domain.RoleOwner, work.GroupID)) {
			return common.ErrForbidden
		}

		query := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID, StateName: c.FromState}).
			Update(&domain.Work{StateName: c.ToState, StateBeginTime: &now})
		if err := query.Error; err != nil {
			return err
		}
		if query.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(query.RowsAffected, 10))
		}

		// update beginProcessTime and endProcessTime
		if work.ProcessBeginTime == nil && toState.Category != state.InBacklog {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_begin_time", &now).Error; err != nil {
				return err
			}
		}
		if work.ProcessEndTime == nil && toState.Category == state.Done {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_end_time", &now).Error; err != nil {
				return err
			}
		} else if work.ProcessEndTime != nil && toState.Category != state.Done {
			if err := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID}).Update("process_end_time", nil).Error; err != nil {
				return err
			}
		}

		// create transition transition
		if err := tx.Create(transition).Error; err != nil {
			return err
		}

		// update process step
		if fromState.Category != state.Done {
			if err := tx.Model(&domain.WorkProcessStep{}).Where(&domain.WorkProcessStep{WorkID: c.WorkID, FlowID: flow.ID, StateName: fromState.Name}).
				Where("end_time is null").Update(&domain.WorkProcessStep{EndTime: &now}).Error; err != nil {
				return err
			}
		}
		if toState.Category != state.Done {
			nextProcessStep := domain.WorkProcessStep{WorkID: work.ID, FlowID: work.FlowID,
				StateName: toState.Name, StateCategory: toState.Category, BeginTime: now}
			if err := tx.Create(nextProcessStep).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return transition, nil
}
