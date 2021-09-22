package workcontribution

import (
	"errors"
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"
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

func QueryWorkContributions(query WorkContributionsQuery, s *session.Session) (*[]WorkContributionRecord, error) {
	records := []WorkContributionRecord{}

	if len(query.WorkKeys) == 0 {
		return &records, nil
	}

	db := persistence.ActiveDataSourceManager.GormDB(s.Context)

	// admin can view all
	if s.Perms.HasRole(account.SystemAdminPermission.ID) {
		if err := db.Where("work_key IN (?)", query.WorkKeys).Find(&records).Error; err != nil {
			return nil, err
		}
		return &records, nil
	}

	// non-admin: group member can view all of project
	if err := db.Where("work_key IN (?) AND work_project_id IN (?)", query.WorkKeys, s.VisibleProjects()).Find(&records).Error; err != nil {
		return nil, err
	}
	return &records, nil
}

func CheckContributorWorkPermission(workKey string, contributorId types.ID, s *session.Session) (*domain.Work, *account.User, error) {
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	work := domain.Work{Identifier: workKey}
	if err := db.Where(&work).First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, bizerror.ErrNoContent // work not exist
		}
		return nil, nil, err
	}
	user := account.User{ID: contributorId}
	if err := db.Where(&user).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, bizerror.ErrNoContent // user not exist
		}
		return nil, nil, err
	}

	if s.Identity.ID != contributorId {
		if !s.Perms.HasRole(account.SystemAdminPermission.ID) && !s.Perms.HasRole(fmt.Sprintf("%s_%d", domain.ProjectRoleManager, work.ProjectID)) {
			return nil, nil, bizerror.ErrForbidden // only system admin and project manager can assign work to other member
		}

		_, contributorVisiableProjects := account.LoadPermFunc(contributorId)
		if !contributorVisiableProjects.HasProject(work.ProjectID) {
			return nil, nil, bizerror.ErrNoContent // contributor is not member of project
		}
	} else {
		if !s.ProjectRoles.HasProject(work.ProjectID) {
			return nil, nil, bizerror.ErrForbidden // session user is not member of project
		}
	}
	return &work, &user, nil
}

func BeginWorkContribution(d *WorkContribution, s *session.Session) (types.ID, error) {
	work, user, err := CheckContributorWorkPermissionFunc(d.WorkKey, d.ContributorId, s)
	if err != nil {
		return 0, err
	}

	var record WorkContributionRecord
	err = persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		condition := map[string]interface{}{
			"work_key":       d.WorkKey,
			"contributor_id": d.ContributorId,
			"checkitem_id":   d.CheckitemId,
		}
		err := tx.Where(condition).First(&record).Error
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			record = WorkContributionRecord{
				ID:               idgen.NextID(idWorker),
				BeginTime:        types.CurrentTimestamp(),
				Effective:        true,
				WorkContribution: *d,
				ContributorName:  user.DisplayName(),
				WorkProjectId:    work.ProjectID,
			}
		} else if err != nil {
			return err
		} else {
			record.ContributorName = user.DisplayName()
			record.WorkProjectId = work.ProjectID
			record.EndTime = types.Timestamp{}
			record.Effective = true
		}

		return tx.Save(&record).Error
	})

	if err != nil {
		return 0, err
	}

	return record.ID, nil
}

func FinishWorkContribution(d *WorkContributionFinishBody, s *session.Session) error {
	work, user, err := CheckContributorWorkPermissionFunc(d.WorkKey, d.ContributorId, s)
	if err != nil {
		return err
	}

	return persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		var record WorkContributionRecord
		condition := map[string]interface{}{
			"work_key":       d.WorkContribution.WorkKey,
			"contributor_id": d.WorkContribution.ContributorId,
			"checkitem_id":   d.WorkContribution.CheckitemId,
		}
		if err := tx.Where(condition).First(&record).Error; err != nil {
			return err
		}

		if record.EndTime.Time().IsZero() {
			record.EndTime = types.CurrentTimestamp()
		}
		record.WorkProjectId = work.ProjectID
		record.ContributorName = user.DisplayName()
		record.Effective = d.Effective

		return tx.Save(&record).Error
	})
}
