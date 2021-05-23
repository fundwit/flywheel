package security

import (
	"errors"
	"flywheel/domain"
	"flywheel/persistence"
	"fmt"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

var (
	systemAdminRole        = Role{ID: "system-admin", Title: "System Administrator"}
	SystemAdminPermission  = Permission{ID: "system:admin", Title: "System Administration"}
	systemAdminRoleBinding = RolePermissionBinding{ID: 1, RoleID: systemAdminRole.ID, PermissionID: SystemAdminPermission.ID}
)

func DefaultSecurityConfiguration() error {
	db := persistence.ActiveDataSourceManager.GormDB()
	if err := db.Save(&systemAdminRole).Error; err != nil {
		return err
	}
	if err := db.Save(&SystemAdminPermission).Error; err != nil {
		return err
	}
	if err := db.Save(&systemAdminRoleBinding).Error; err != nil {
		return err
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		admin := User{}
		err := tx.Model(&User{}).Where(&User{ID: 1}).First(&admin).Error
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Save(&User{ID: 1, Name: "admin", Secret: HashSha256("admin123")}).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(&UserRoleBinding{ID: 1, UserID: 1, RoleID: systemAdminRole.ID}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// as a simple initial solution, we use project member relationship as the metadata of permissions
func LoadPerms(uid types.ID) ([]string, []domain.ProjectRole) {
	var roles []string
	db := persistence.ActiveDataSourceManager.GormDB()

	// system perms
	var systemRoles []string
	if err := db.Model(&UserRoleBinding{}).Where(&UserRoleBinding{UserID: uid}).Pluck("role_id", &systemRoles).Error; err != nil {
		panic(err)
	}
	if len(systemRoles) > 0 {
		var systemPerms []string
		if err := db.Model(&RolePermissionBinding{}).Where("role_id IN (?)", systemRoles).Pluck("permission_id", &systemPerms).Error; err != nil {
			panic(err)
		}
		for _, perm := range systemPerms {
			roles = append(roles, perm)
		}
	}

	// project role
	var gms []domain.ProjectMember
	if err := db.Model(&domain.ProjectMember{}).Where(&domain.ProjectMember{MemberId: uid}).Scan(&gms).Error; err != nil {
		panic(err)
	}

	var projectRoles []domain.ProjectRole
	var projectIds []types.ID
	for _, gm := range gms {
		roles = append(roles, fmt.Sprintf("%s_%d", gm.Role, gm.ProjectId))
		projectRoles = append(projectRoles, domain.ProjectRole{Role: gm.Role, ProjectID: gm.ProjectId})
		projectIds = append(projectIds, gm.ProjectId)
	}

	m := map[types.ID]domain.Project{}
	if len(projectIds) > 0 {
		var projects []domain.Project
		if err := persistence.ActiveDataSourceManager.GormDB().Model(&domain.Project{}).Where("id in (?)", projectIds).Scan(&projects).Error; err != nil {
			panic(err)
		}
		for _, project := range projects {
			m[project.ID] = project
		}
	}

	for i := 0; i < len(projectRoles); i++ {
		projectRole := projectRoles[i]

		project := m[projectRole.ProjectID]
		if (project == domain.Project{}) {
			panic(errors.New("project " + projectRole.ProjectID.String() + " is not exist"))
		}

		projectRole.ProjectName = project.Name
		projectRole.ProjectIdentifier = project.Identifier

		projectRoles[i] = projectRole
	}

	if roles == nil {
		roles = []string{}
	}
	if projectRoles == nil {
		projectRoles = []domain.ProjectRole{}
	}
	return roles, projectRoles
}
