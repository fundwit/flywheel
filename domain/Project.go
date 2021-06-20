package domain

import (
	"time"

	"github.com/fundwit/go-commons/types"
)

type Project struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	Identifier string `json:"identifier"  gorm:"unique_index:identifier_unique"`
	Name       string `json:"name" gorm:"unique_index:name_idx"`

	NextWorkId int `json:"nextWorkId" sql:"type:BIGINT UNSIGNED NOT NULL"`

	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
	Creator    types.ID  `json:"creator"`
}

type ProjectCreating struct {
	Name       string `json:"name" binding:"required,lte=60"`
	Identifier string `json:"identifier" binding:"required,lte=6,uppercase"`
}

type ProjectUpdating struct {
	Name string `json:"name" binding:"required,lte=60"`
}

const ProjectRoleManager = "manager"
const ProjectRoleCommon = "common"
