package domain

import (
	"flywheel/domain/worktype"
	"flywheel/persistence"
	"flywheel/utils"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

type WorkManagerTraits interface {
	QueryWork() (*[]Work, error)
	WorkDetail(id utils.ID) (*WorkDetail, error)
	CreateWork(c *WorkCreation) (*WorkDetail, error)
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

func (m *WorkManager) QueryWork() (*[]Work, error) {
	var works []Work
	db := m.dataSource.GormDB()

	if err := db.Model(Work{}).Find(&works).Error; err != nil {
		return nil, err
	}

	return &works, nil
}

func (m *WorkManager) WorkDetail(id utils.ID) (*WorkDetail, error) {
	workDetail := WorkDetail{}
	db := m.dataSource.GormDB()

	if err := db.Where(&Work{ID: id}).First(&(workDetail.Work)).Error; err != nil {
		return nil, err
	}

	// load type and state
	gwt := worktype.GenericWorkType

	workDetail.Type = gwt.WorkTypeBase
	state, err := gwt.FindState(workDetail.StateName)
	if err != nil {
		return nil, err
	}
	workDetail.State = state

	return &workDetail, nil
}

func (m *WorkManager) CreateWork(c *WorkCreation) (*WorkDetail, error) {
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
	if err := db.Delete(Work{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}
