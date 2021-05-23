package namespace

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"fmt"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"time"
)

var (
	idWorker = sonyflake.NewSonyflake(sonyflake.Settings{})
)

func QueryProjects(sec *security.Context) (*[]domain.Project, error) {
	if !sec.HasRole(security.SystemAdminPermission.ID) {
		return nil, bizerror.ErrForbidden
	}

	var groups []domain.Project
	if err := persistence.ActiveDataSourceManager.GormDB().Find(&groups).Error; err != nil {
		return nil, err
	}
	return &groups, nil
}

func CreateProject(c *domain.ProjectCreating, sec *security.Context) (*domain.Project, error) {
	if !sec.HasRole(security.SystemAdminPermission.ID) {
		return nil, bizerror.ErrForbidden
	}

	now := time.Now()
	g := domain.Project{ID: common.NextId(idWorker), Name: c.Name, Identifier: c.Identifier, NextWorkId: 1, CreateTime: now, Creator: sec.Identity.ID}
	r := domain.ProjectMember{ProjectId: g.ID, MemberId: sec.Identity.ID, Role: domain.RoleOwner, CreateTime: now}
	err := persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(g).Error; err != nil {
			return err
		}
		if err := tx.Create(r).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func UpdateProject(id types.ID, d *domain.ProjectUpdating, sec *security.Context) error {
	if !sec.HasRole(security.SystemAdminPermission.ID) {
		return bizerror.ErrForbidden
	}

	return persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		var group domain.Project
		if err := tx.Where(domain.Project{ID: id}).First(&group).Error; err != nil {
			return err
		}

		if err := tx.Model(&domain.Project{ID: id}).Where(domain.Project{ID: id}).Update(domain.Project{Name: d.Name}).Error; err != nil {
			return err
		}
		return nil
	})
}

func QueryProjectRole(projectId types.ID, sec *security.Context) (string, error) {
	gm := domain.ProjectMember{ProjectId: projectId, MemberId: sec.Identity.ID}
	db := persistence.ActiveDataSourceManager.GormDB()
	var founds []domain.ProjectMember
	if err := db.Model(domain.ProjectMember{}).Where(&gm).Find(&founds).Error; err != nil || founds == nil || len(founds) == 0 {
		return "", err
	}
	return founds[0].Role, nil
}

func NextWorkIdentifier(projectId types.ID, tx *gorm.DB) (string, error) {
	group := domain.Project{}
	if err := tx.Where(&domain.Project{ID: projectId}).First(&group).Error; err != nil {
		return "", err
	}

	// consume current value
	nextWorkID := fmt.Sprintf("%s-%d", group.Identifier, group.NextWorkId)
	// generate next value
	db := tx.Model(&domain.Project{}).Where(&domain.Project{ID: projectId, NextWorkId: group.NextWorkId}).
		Update("next_work_id", group.NextWorkId+1)
	if db.Error != nil {
		return "", db.Error
	}
	if db.RowsAffected != 1 {
		return "", errors.New("concurrent modification")
	}
	return nextWorkID, nil
}


func QueryProjectNames (ids []types.ID) (map[types.ID]string, error) {
	if len(ids) == 0 {
		return map[types.ID]string{}, nil
	}
	db := persistence.ActiveDataSourceManager.GormDB()
	var records []domain.Project
	if err := db.Model(&domain.Project{}).Where("id IN (?)", ids).Find(&records).Error; err != nil {
		return nil, err
	}
	result := map[types.ID]string{}

	for _, r := range records {
		result[r.ID] = r.Name
	}
	return result, nil
}