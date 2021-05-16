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

// as a simple initial solution, we use group member relationship as the metadata of permissions
func LoadPerms(uid types.ID) ([]string, []domain.GroupRole) {
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

	// group role
	var gms []domain.GroupMember
	if err := db.Model(&domain.GroupMember{}).Where(&domain.GroupMember{MemberId: uid}).Scan(&gms).Error; err != nil {
		panic(err)
	}

	var groupRoles []domain.GroupRole
	var groupIds []types.ID
	for _, gm := range gms {
		roles = append(roles, fmt.Sprintf("%s_%d", gm.Role, gm.GroupID))
		groupRoles = append(groupRoles, domain.GroupRole{Role: gm.Role, GroupID: gm.GroupID})
		groupIds = append(groupIds, gm.GroupID)
	}

	m := map[types.ID]domain.Group{}
	if len(groupIds) > 0 {
		var groups []domain.Group
		if err := persistence.ActiveDataSourceManager.GormDB().Model(&domain.Group{}).Where("id in (?)", groupIds).Scan(&groups).Error; err != nil {
			panic(err)
		}
		for _, group := range groups {
			m[group.ID] = group
		}
	}

	for i := 0; i < len(groupRoles); i++ {
		groupRole := groupRoles[i]

		group := m[groupRole.GroupID]
		if (group == domain.Group{}) {
			panic(errors.New("group " + groupRole.GroupID.String() + " is not exist"))
		}

		groupRole.GroupName = group.Name
		groupRole.GroupIdentifier = group.Identifier

		groupRoles[i] = groupRole
	}

	if roles == nil {
		roles = []string{}
	}
	if groupRoles == nil {
		groupRoles = []domain.GroupRole{}
	}
	return roles, groupRoles
}
