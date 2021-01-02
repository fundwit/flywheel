package flow

import (
	"errors"
	"flywheel/domain"

	"flywheel/persistence"
	"flywheel/utils"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"strconv"
	"time"
)

type WorkflowManagerTraits interface {
	CreateWorkStateTransition(*WorkStateTransitionBrief) (*WorkStateTransition, error)
}

type WorkflowManager struct {
	dataSource *persistence.DataSourceManager
	idWorker   *sonyflake.Sonyflake
}

func NewWorkflowManager(ds *persistence.DataSourceManager) WorkflowManagerTraits {
	return &WorkflowManager{
		dataSource: ds,
		idWorker:   sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *WorkflowManager) CreateWorkStateTransition(c *WorkStateTransitionBrief) (*WorkStateTransition, error) {
	flow := domain.FindWorkflow(c.FlowID)
	if flow == nil {
		return nil, errors.New("workflow " + strconv.FormatUint(uint64(c.FlowID), 10) + " not found")
	}
	// check whether the transition is acceptable
	availableTransitions := flow.StateMachine.AvailableTransitions(c.FromState, c.ToState)
	if len(availableTransitions) != 1 {
		return nil, errors.New("transition from " + c.FromState + " to " + c.ToState + " is not invalid")
	}

	id, err := m.idWorker.NextID()
	if err != nil {
		return nil, err
	}
	record := &WorkStateTransition{ID: utils.ID(id), CreateTime: time.Now(), WorkStateTransitionBrief: *c}

	db := m.dataSource.GormDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// update work.stageName
		query := tx.Model(&domain.Work{}).Where(&domain.Work{ID: c.WorkID, StateName: c.FromState}).Update(&domain.Work{StateName: c.ToState})
		if err := query.Error; err != nil {
			return err
		}
		if query.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(query.RowsAffected, 10))
		}
		// create transition record
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}
