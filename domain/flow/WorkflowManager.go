package flow

import (
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/state"
	"flywheel/persistence"
	"flywheel/security"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"time"
)

type WorkflowManagerTraits interface {
	QueryWorkflows(query *domain.WorkflowQuery, sec *security.Context) (*[]domain.Workflow, error)
	CreateWorkflow(c *WorkflowCreation, sec *security.Context) (*domain.WorkflowDetail, error)
	DetailWorkflow(ID types.ID, sec *security.Context) (*domain.WorkflowDetail, error)
	UpdateWorkflowBase(ID types.ID, c *WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error)
	DeleteWorkflow(ID types.ID, sec *security.Context) error
	// updateStateMachine
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

type WorkflowBaseUpdation struct {
	Name       string `json:"name"     binding:"required"`
	ThemeColor string `json:"themeColor" binding:"required"`
	ThemeIcon  string `json:"themeIcon"  binding:"required"`
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
			CreateTime: time.Now().Round(time.Millisecond),
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
		if err := tx.Where(&domain.Workflow{ID: id}).First(&(workflowDetail.Workflow)).Error; err != nil {
			return err
		}
		if !sec.HasRoleSuffix("_" + workflowDetail.GroupID.String()) {
			return common.ErrForbidden
		}

		var stateRecords []domain.WorkflowState
		if err := tx.Where(domain.WorkflowState{WorkflowID: workflowDetail.ID}).Order("`order` ASC").Find(&stateRecords).Error; err != nil {
			return err
		}
		var transitionRecords []domain.WorkflowStateTransition
		if err := tx.Where(domain.WorkflowStateTransition{WorkflowID: workflowDetail.ID}).Find(&transitionRecords).Error; err != nil {
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

func (m *WorkflowManager) UpdateWorkflowBase(id types.ID, c *WorkflowBaseUpdation, sec *security.Context) (*domain.Workflow, error) {
	wf := domain.Workflow{}
	db := m.dataSource.GormDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&domain.Workflow{ID: id}).First(&wf).Error; err != nil {
			return err
		}
		if !sec.HasRoleSuffix("owner_" + wf.GroupID.String()) {
			return common.ErrForbidden
		}
		if err := tx.Model(&domain.Workflow{}).Where(&domain.Workflow{ID: id}).
			Update(&domain.Workflow{Name: c.Name, ThemeIcon: c.ThemeIcon, ThemeColor: c.ThemeColor}).Error; err != nil {
			return err
		}
		// query again
		if err := tx.Where(&domain.Workflow{ID: id}).First(&wf).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

func (m *WorkflowManager) DeleteWorkflow(id types.ID, sec *security.Context) error {
	wf := domain.Workflow{}
	db := m.dataSource.GormDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&domain.Workflow{ID: id}).First(&wf).Error; err != nil {
			return err
		}
		if !sec.HasRoleSuffix("owner_" + wf.GroupID.String()) {
			return common.ErrForbidden
		}

		if err := isWorkflowReferenced(tx, wf.ID); err != nil {
			return err
		}

		if err := tx.Model(&domain.Workflow{}).Delete(&domain.Workflow{ID: id}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.WorkflowState{}).Where("workflow_id = ?", wf.ID).
			Delete(&domain.WorkflowState{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.WorkflowStateTransition{}).Where("workflow_id = ?", wf.ID).
			Delete(&domain.WorkflowStateTransition{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

func isWorkflowReferenced(db *gorm.DB, workflowID types.ID) error {
	var work domain.Work
	err := db.Model(&domain.Work{}).Where(&domain.Work{FlowID: workflowID}).First(&work).Error
	if err == nil {
		return common.ErrWorkflowIsReferenced
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	var workProcessStep domain.WorkProcessStep
	err = db.Model(&domain.WorkProcessStep{}).Where(&domain.WorkProcessStep{FlowID: workflowID}).First(&workProcessStep).Error
	if err == nil {
		return common.ErrWorkflowIsReferenced
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	var workStateTransition domain.WorkStateTransition
	err = db.Model(&domain.WorkStateTransition{}).Where(&domain.WorkStateTransition{WorkStateTransitionBrief: domain.WorkStateTransitionBrief{FlowID: workflowID}}).
		First(&workStateTransition).Error
	if err == nil {
		return common.ErrWorkflowIsReferenced
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}
