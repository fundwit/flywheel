package work

import (
	"errors"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/persistence"
	"flywheel/security"
	"fmt"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"strconv"
)

type WorkManagerTraits interface {
	QueryWork(query *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error)
	WorkDetail(id types.ID, sec *security.Context) (*domain.WorkDetail, error)
	CreateWork(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error)
	UpdateWork(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error)
	DeleteWork(id types.ID, sec *security.Context) error
	UpdateStateRangeOrders(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error
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

func (m *WorkManager) QueryWork(query *domain.WorkQuery, sec *security.Context) (*[]domain.Work, error) {
	var works []domain.Work
	db := m.dataSource.GormDB()

	q := db.Where(domain.Work{GroupID: query.GroupID})
	if query.Name != "" {
		q = q.Where("name like ?", "%"+query.Name+"%")
	}
	visibleGroups := sec.VisibleGroups()
	if len(visibleGroups) == 0 {
		return &[]domain.Work{}, nil
	}
	q = q.Where("group_id in (?)", visibleGroups).Order("order_in_state ASC")
	if err := q.Find(&works).Error; err != nil {
		return nil, err
	}
	return &works, nil
}

func (m *WorkManager) WorkDetail(id types.ID, sec *security.Context) (*domain.WorkDetail, error) {
	workDetail := domain.WorkDetail{}
	db := m.dataSource.GormDB()
	if err := db.Where(&domain.Work{ID: id}).First(&(workDetail.Work)).Error; err != nil {
		return nil, err
	}

	if !sec.HasRoleSuffix("_" + workDetail.GroupID.String()) {
		return nil, errors.New("forbidden")
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

func (m *WorkManager) CreateWork(c *domain.WorkCreation, sec *security.Context) (*domain.WorkDetail, error) {
	if !sec.HasRoleSuffix("_" + c.GroupID.String()) {
		return nil, errors.New("forbidden")
	}

	workDetail := c.BuildWorkDetail(common.NextId(m.idWorker))

	db := m.dataSource.GormDB()
	err := db.Transaction(func(tx *gorm.DB) error {
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

func (m *WorkManager) UpdateWork(id types.ID, u *domain.WorkUpdating, sec *security.Context) (*domain.Work, error) {
	if err := m.checkPerms(id, sec); err != nil {
		return nil, err
	}

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

func (m *WorkManager) UpdateStateRangeOrders(wantedOrders *[]domain.StageRangeOrderUpdating, sec *security.Context) error {
	if wantedOrders == nil || len(*wantedOrders) == 0 {
		return nil
	}

	return m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		for _, orderUpdating := range *wantedOrders {
			if err := m.checkPerms(orderUpdating.ID, sec); err != nil {
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
		}
		return nil
	})
}

func (m *WorkManager) DeleteWork(id types.ID, sec *security.Context) error {
	if err := m.checkPerms(id, sec); err != nil {
		return err
	}

	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(flow.WorkStateTransition{}, "work_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})

	return err
}

func (m *WorkManager) checkPerms(id types.ID, sec *security.Context) error {
	var work domain.Work
	if err := m.dataSource.GormDB().Where(&domain.Work{ID: id}).First(&work).Error; err != nil {
		return err
	}
	if sec == nil || !sec.HasRoleSuffix("_"+work.GroupID.String()) {
		return errors.New("forbidden")
	}
	return nil
}
