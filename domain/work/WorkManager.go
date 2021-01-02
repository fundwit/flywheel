package work

import (
	"errors"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/utils"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"strconv"
)

type WorkManagerTraits interface {
	QueryWork() (*[]domain.Work, error)
	WorkDetail(id utils.ID) (*domain.WorkDetail, error)
	CreateWork(c *domain.WorkCreation) (*domain.WorkDetail, error)
	UpdateWork(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error)
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

func (m *WorkManager) UpdateWork(id utils.ID, u *domain.WorkUpdating) (*domain.Work, error) {
	var work domain.Work
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &work, nil
}

func (m *WorkManager) DeleteWork(id utils.ID) error {
	db := m.dataSource.GormDB()
	if err := db.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}
