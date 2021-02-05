package domain

import (
	"github.com/fundwit/go-commons/types"
	"time"
)

type Group struct {
	ID         types.ID  `json:"id" gorm:"primary_key"`
	Name       string    `json:"group" gorm:"unique_index:name_idx"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
	Creator    types.ID  `json:"creator"`
}

type GroupMember struct {
	GroupID    types.ID  `json:"groupId"  gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	MemberId   types.ID  `json:"memberId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Role       string    `json:"role"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

const RoleOwner = "owner"
