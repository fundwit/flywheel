package workcontribution

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
)

var (
	CheckContributorWorkPermissionFunc = CheckContributorWorkPermission
	idWorker                           = sonyflake.NewSonyflake(sonyflake.Settings{})
)

type WorkContributionsQuery struct {
	WorkKeys []string `form:"workKey" json:"workKeys"`
}

func QueryWorkContributions(query WorkContributionsQuery, sec *security.Context) (*[]WorkContributionRecord, error) {
	records := []WorkContributionRecord{}

	if len(query.WorkKeys) == 0 {
		return &records, nil
	}

	db := persistence.ActiveDataSourceManager.GormDB()

	// admin can view all
	if sec.HasRole(security.SystemAdminPermission.ID) {
		if err := db.Where("work_key IN (?)", query.WorkKeys).Find(&records).Error; err != nil {
			return nil, err
		}
		return &records, nil
	}

	// non-admin: group member can view all of project
	if err := db.Where("work_key IN (?) AND work_project_id IN (?)", query.WorkKeys, sec.VisibleProjects()).Find(&records).Error; err != nil {
		return nil, err
	}
	return &records, nil
}

func CheckContributorWorkPermission(workKey string, contributorId types.ID, sec *security.Context) (*domain.Work, *security.User, error) {
	db := persistence.ActiveDataSourceManager.GormDB()
	work := domain.Work{Identifier: workKey}
	if err := db.Where(&work).First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, bizerror.ErrNoContent // work not exist
		}
		return nil, nil, err
	}
	user := security.User{ID: contributorId}
	if err := db.Where(&user).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, bizerror.ErrNoContent // user not exist
		}
		return nil, nil, err
	}

	if sec.Identity.ID != contributorId {
		if !sec.HasRole(security.SystemAdminPermission.ID) && !sec.HasRole(fmt.Sprintf("%s_%d", domain.ProjectRoleManager, work.ProjectID)) {
			return nil, nil, bizerror.ErrForbidden // only system admin and project manager can assign work to other member
		}

		_, contributorVisiableProjects := security.LoadPermFunc(contributorId)
		if !contributorVisiableProjects.HasProject(work.ProjectID) {
			return nil, nil, bizerror.ErrNoContent // contributor is not member of project
		}
	} else {
		if !sec.ProjectRoles.HasProject(work.ProjectID) {
			return nil, nil, bizerror.ErrForbidden // session user is not member of project
		}
	}
	return &work, &user, nil
}

func BeginWorkContribution(d *WorkContribution, sec *security.Context) (types.ID, error) {
	work, user, err := CheckContributorWorkPermissionFunc(d.WorkKey, d.ContributorId, sec)
	if err != nil {
		return 0, err
	}

	var record WorkContributionRecord
	err = persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		err := tx.Where(&WorkContributionRecord{WorkContribution: *d}).First(&record).Error
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			record = WorkContributionRecord{
				ID:               common.NextID(idWorker),
				BeginTime:        common.CurrentTimestamp(),
				Effective:        true,
				WorkContribution: *d,
				ContributorName:  user.Name,
				WorkProjectId:    work.ProjectID,
			}
		} else if err != nil {
			return err
		} else {
			record.ContributorName = user.Name
			record.WorkProjectId = work.ProjectID
			record.EndTime = common.Timestamp{}
			record.Effective = true
		}

		return tx.Save(&record).Error
	})

	if err != nil {
		return 0, err
	}

	return record.ID, nil
}

func FinishWorkContribution(d *WorkContribuitonFinishBody, sec *security.Context) error {
	work, user, err := CheckContributorWorkPermissionFunc(d.WorkKey, d.ContributorId, sec)
	if err != nil {
		return err
	}

	return persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		var record WorkContributionRecord
		if err := tx.Where(&WorkContributionRecord{WorkContribution: d.WorkContribution}).First(&record).Error; err != nil {
			return err
		}

		if record.EndTime.Time().IsZero() {
			record.EndTime = common.CurrentTimestamp()
		}
		record.WorkProjectId = work.ProjectID
		record.ContributorName = user.Name
		record.Effective = d.Effective

		return tx.Save(&record).Error
	})
}
