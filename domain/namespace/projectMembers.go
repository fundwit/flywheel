package namespace

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"fmt"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

var (
	QueryProjectNamesFunc    = QueryProjectNames
	QueryAccountNamesFunc    = security.QueryAccountNames
	DetailProjectMembersFunc = DetailProjectMembers
)

func CreateProjectMember(d *domain.ProjectMemberCreation, sec *security.Context) error {
	return persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		if !sec.HasRole(security.SystemAdminPermission.ID) && !sec.HasRole(fmt.Sprintf("%s_%d", domain.RoleOwner, d.ProjectID)) {
			return bizerror.ErrForbidden
		}

		project := domain.Project{ID: d.ProjectID}
		if err := tx.Model(&domain.Project{}).Where(&project).First(&project).Error; err != nil {
			return err
		}

		user := security.User{ID: d.MemberId}
		if err := tx.Model(&security.User{}).Where(&user).First(&user).Error; err != nil {
			return err
		}

		// only allow one owner in one project, create new owner will delete the old one
		if d.Role == domain.RoleOwner {
			if err := tx.Where("project_id = ? AND `role` LIKE ?", d.ProjectID, domain.RoleOwner).Delete(&domain.ProjectMember{}).Error; err != nil {
				return err
			}
		}

		// update when exist
		record := domain.ProjectMember{ProjectId: d.ProjectID, MemberId: d.MemberId, Role: d.Role, CreateTime: time.Now()}
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		return nil
	})
}

func QueryProjectMemberDetails(d *domain.ProjectMemberQuery, sec *security.Context) (*[]domain.ProjectMemberDetail, error) {
	dbQuery := persistence.ActiveDataSourceManager.GormDB().Model(&domain.ProjectMember{})

	if !sec.HasRole(security.SystemAdminPermission.ID) {
		dbQuery = dbQuery.Where("project_id IN (?)", sec.VisibleProjects())
	}
	if d.ProjectID != nil {
		dbQuery = dbQuery.Where("project_id = ?", d.ProjectID)
	}
	if d.MemberID != nil {
		dbQuery = dbQuery.Where("member_id = ?", d.MemberID)
	}

	var result []domain.ProjectMember
	if err := dbQuery.Find(&result).Error; err != nil {
		return nil, err
	}

	details, err := DetailProjectMembersFunc(&result)
	if err != nil {
		return nil, err
	}

	return details, nil
}

func DetailProjectMembers(pms *[]domain.ProjectMember) (*[]domain.ProjectMemberDetail, error) {
	if pms == nil {
		return &[]domain.ProjectMemberDetail{}, nil
	}

	var projectIds []types.ID
	var memberIds []types.ID

	for _, pm := range *pms {
		projectIds = append(projectIds, pm.ProjectId)
		memberIds = append(memberIds, pm.MemberId)
	}

	projectIdNameMap, err := QueryProjectNamesFunc(projectIds)
	if err != nil {
		return nil, err
	}
	memberIdNameMap, err := QueryAccountNamesFunc(memberIds)
	if err != nil {
		return nil, err
	}

	var details []domain.ProjectMemberDetail
	for _, pm := range *pms {
		detail := domain.ProjectMemberDetail{ProjectMember: pm, ProjectName: "Unknown", MemberName: "Unknown"}
		projectName, found := projectIdNameMap[pm.ProjectId]
		if found {
			detail.ProjectName = projectName
		}
		accountName, found := memberIdNameMap[pm.MemberId]
		if found {
			detail.MemberName = accountName
		}

		details = append(details, detail)
	}

	return &details, nil
}

func DeleteProjectMember(d *domain.ProjectMemberDeletion, sec *security.Context) error {
	if !sec.HasRole(security.SystemAdminPermission.ID) && !sec.HasRole(fmt.Sprintf("%s_%d", domain.RoleOwner, d.ProjectID)) {
		return bizerror.ErrForbidden
	}

	return persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		record := domain.ProjectMember{}
		if err := tx.Where("project_id = ? AND member_id = ?", d.ProjectID, d.MemberID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		if record.Role == domain.RoleOwner {
			return bizerror.ErrProjectOwnerDelete
		}

		return tx.Where("project_id = ? AND member_id = ?", d.ProjectID, d.MemberID).Delete(&domain.ProjectMember{}).Error
	})
}
