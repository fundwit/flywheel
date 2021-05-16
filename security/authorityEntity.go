package security

import "github.com/fundwit/go-commons/types"

type Role struct {
	ID    string `json:"id" gorm:"primary_key"`
	Title string `json:"title"`
}

type UserRoleBinding struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	UserID types.ID `json:"userId" gorm:"unique_index:uni_user_role"`
	RoleID string   `json:"roleId" gorm:"unique_index:uni_user_role"`
}

type Permission struct {
	ID    string `json:"id" gorm:"primary_key"`
	Title string `json:"title"`
}

type RolePermissionBinding struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	RoleID       string `json:"roleId" gorm:"unique_index:uni_role_perm"`
	PermissionID string `json:"permissionId" gorm:"unique_index:uni_role_perm"`
}
