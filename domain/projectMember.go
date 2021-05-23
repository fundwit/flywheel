package domain

import (
	"time"

	"github.com/fundwit/go-commons/types"
)

type ProjectMember struct {
	ProjectId types.ID `json:"projectId"  gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	MemberId  types.ID `json:"memberId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`

	Role       string    `json:"role"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

type ProjectMemberDetail struct {
	ProjectMember

	ProjectName string `json:"projectName"`

	MemberName string `json:"memberName"`
}

type ProjectMemberCreation struct {
	ProjectID types.ID `json:"projectId"`
	MemberId  types.ID `json:"memberId"`
	Role      string   `json:"role"`
}

type ProjectMemberQuery struct {
	ProjectID *types.ID `form:"projectId"`
	MemberID  *types.ID `form:"memberId"`
}

type ProjectMemberDeletion struct {
	ProjectID types.ID `form:"projectId" binding:"required"`
	MemberID  types.ID `form:"memberId" binding:"required"`
}
