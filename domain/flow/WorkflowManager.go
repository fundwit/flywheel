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
	QueryWorkflows(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error)
	CreateWorkflow(c *WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error)
	DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error)

	// update
	// delete

	CreateWorkStateTransition(*domain.WorkStateTransitionBrief, *security.Context) (*domain.WorkStateTransition, error)
}

type WorkflowManager struct {
	dataSource *persistence.DataSourceManager
	idWorker   *sonyflake.Sonyflake
}

type WorkflowCreation struct {
	Name       string   `json:"name"     binding:"required"`
	GroupID    types.ID `json:"groupId" binding:"required"`
	ThemeColor string   `json:"themeColor" binding:"required"`
	ThemeIcon  string   `json:"themeIcon"  binding:"required"`

	StateMachine state.StateMachine `json:"stateMachine" binding:"required"`
}

func NewWorkflowManager(ds *persistence.DataSourceManager) *WorkflowManager {
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
			ThemeColor: c.ThemeColor,
			ThemeIcon:  c.ThemeIcon,
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

func (m *WorkflowManager) DetailWorkflow(id types.ID, sec *security.Context) (*domain.WorkflowDetail, error) {
	workflowDetail := domain.WorkflowDetail{}
	db := m.dataSource.GormDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := db.Where(&domain.Workflow{ID: id}).First(&(workflowDetail.Workflow)).Error; err != nil {
			return err
		}
		if !sec.HasRoleSuffix("_" + workflowDetail.GroupID.String()) {
			return common.ErrForbidden
		}

		var stateRecords []domain.WorkflowState
		if err := db.Where(domain.WorkflowState{WorkflowID: workflowDetail.ID}).Order("`order` ASC").Find(&stateRecords).Error; err != nil {
			return err
		}
		var transitionRecords []domain.WorkflowStateTransition
		if err := db.Where(domain.WorkflowStateTransition{WorkflowID: workflowDetail.ID}).Find(&transitionRecords).Error; err != nil {
			return err
		}
		stateMachine := state.StateMachine{}
		for _, record := range stateRecords {
			stateMachine.States = append(stateMachine.States, state.State{Name: record.Name, Category: record.Category})
		}
		for _, record := range transitionRecords {
			from, fromStateFound := stateMachine.FindState(record.FromState)
			to, toStateFound := stateMachine.FindState(record.ToState)
			if !fromStateFound || !toStateFound {
				return domain.ErrInvalidState
			}
			stateMachine.Transitions = append(stateMachine.Transitions, state.Transition{Name: record.Name, From: from, To: to})
		}

		workflowDetail.StateMachine = stateMachine
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &workflowDetail, nil
}

func (m *WorkflowManager) QueryWorkflows(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error) {
	var workflows []domain.Workflow
	db := m.dataSource.GormDB()

	q := db.Where(domain.Workflow{GroupID: query.GroupID})
	if query.Name != "" {
		q = q.Where("name like ?", "%"+query.Name+"%")
	}
	visibleGroups := sec.VisibleGroups()
	if len(visibleGroups) == 0 {
		return &[]domain.Workflow{}, nil
	}
	q = q.Where("group_id in (?)", visibleGroups)
	if err := q.Find(&workflows).Error; err != nil {
		return nil, err
	}

	return &workflows, nil
}

func (m *WorkflowManager) CreateWorkStateTransition(c *domain.WorkStateTransitionBrief, sec *security.Context) (*domain.WorkStateTransition, error) {
	flow, err := m.DetailWorkflow(c.FlowID, sec)
	if err != nil {
		return nil, err
	}
	// check whether the transition is acceptable
	availableTransitions := flow.StateMachine.AvailableTransitions(c.FromState, c.ToState)
	if len(availableTransitions) != 1 {
		return nil, errors.New("transition from " + c.FromState + " to " + c.ToState + " is not invalid")
	}

	now := time.Now()
	newId := common.NextId(m.idWorker)
	transition := &domain.WorkStateTransition{ID: newId, CreateTime: now, Creator: sec.Identity.ID, WorkStateTransitionBrief: *c}

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
			Update(&domain.Work{StateName: c.ToState, StateCategory: toState.Category, StateBeginTime: &now})
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
