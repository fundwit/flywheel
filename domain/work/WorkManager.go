package work

import (
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/utils"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

type WorkManagerTraits interface {
	QueryWork() (*[]domain.Work, error)
	WorkDetail(id utils.ID) (*domain.WorkDetail, error)
	CreateWork(c *domain.WorkCreation) (*domain.WorkDetail, error)
	DeleteWork(id utils.ID) error
}

type WorkManager struct {
	dataSource *persistence.DataSourceManager
	idWorker   *sonyflake.Sonyflake
}

func NewWorkManager(ds *persistence.DataSourceManager) *WorkManager {
	return &WorkManager{
		dataSource: ds,
		idWorker:   sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *WorkManager) QueryWork() (*[]domain.Work, error) {
	var works []domain.Work
	db := m.dataSource.GormDB()

	if err := db.Model(domain.Work{}).Find(&works).Error; err != nil {
		return nil, err
	}

	return &works, nil
}

func (m *WorkManager) WorkDetail(id utils.ID) (*domain.WorkDetail, error) {
	workDetail := domain.WorkDetail{}
	db := m.dataSource.GormDB()

	if err := db.Where(&domain.Work{ID: id}).First(&(workDetail.Work)).Error; err != nil {
		return nil, err
	}

	// load type and state
	gwt := domain.GenericWorkFlow

	workDetail.Type = gwt.WorkFlowBase
	state, found := gwt.FindState(workDetail.StateName)
	if !found {
		return nil, fmt.Errorf("invalid state '%s'", workDetail.StateName)
	}
	workDetail.State = state

	return &workDetail, nil
}

func (m *WorkManager) CreateWork(c *domain.WorkCreation) (*domain.WorkDetail, error) {
	id, err := m.idWorker.NextID()
	if err != nil {
		return nil, err
	}
	workDetail := c.BuildWorkDetail(utils.ID(id))

	db := m.dataSource.GormDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(workDetail.Work).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return workDetail, nil
}

func (m *WorkManager) DeleteWork(id utils.ID) error {
	db := m.dataSource.GormDB()
	if err := db.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}
