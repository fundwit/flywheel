package domain

import (
	"github.com/fundwit/go-commons/types"
	"time"
)

type Group struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	Identifier string `json:"identifier"  gorm:"unique_index:identifier_unique"`
	Name       string `json:"name" gorm:"unique_index:name_idx"`

	NextWorkId int `json:"nextWorkId" sql:"type:BIGINT UNSIGNED NOT NULL"`

	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
	Creator    types.ID  `json:"creator"`
}

type GroupMember struct {
	GroupID    types.ID  `json:"groupId"  gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	MemberId   types.ID  `json:"memberId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Role       string    `json:"role"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

type GroupCreating struct {
	Name       string `json:"name" binding:"required,lte=60"`
	Identifier string `json:"identifier" binding:"required,lte=6,uppercase"`
}

type GroupUpdating struct {
	Name string `json:"name" binding:"required,lte=60"`
}

const RoleOwner = "owner"
