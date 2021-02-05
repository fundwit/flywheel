package work

import (
	"errors"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"github.com/jinzhu/gorm"
)

type WorkProcessManagerTraits interface {
	QueryProcessSteps(query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error)
}

type WorkProcessManager struct {
	dataSource *persistence.DataSourceManager
}

func NewWorkProcessManager(ds *persistence.DataSourceManager) *WorkProcessManager {
	return &WorkProcessManager{
		dataSource: ds,
	}
}

func (m *WorkProcessManager) QueryProcessSteps(query *domain.WorkProcessStepQuery, sec *security.Context) (*[]domain.WorkProcessStep, error) {
	db := m.dataSource.GormDB()
	work := domain.Work{}
	if err := db.Where(&domain.Work{ID: query.WorkID}).Select("group_id").First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &[]domain.WorkProcessStep{}, nil
		} else {
			return nil, err
		}
	}
	if !sec.HasRoleSuffix("_" + work.GroupID.String()) {
		return &[]domain.WorkProcessStep{}, nil
	}

	var processSteps []domain.WorkProcessStep
	if err := db.Where(&domain.WorkProcessStep{WorkID: query.WorkID}).Find(&processSteps).Error; err != nil {
		return nil, err
	}
	return &processSteps, nil
}
