package domain

import (
	"flywheel/domain/worktype"
	"flywheel/utils"
	"time"
)

type Work struct {
	ID         utils.ID  `json:"id"`
	Name       string    `json:"name"`
	Group      string    `json:"group"`
	TypeID     utils.ID  `json:"typeId"`
	CreateTime time.Time `json:"createTime"`

	StateName string `json:"stateName"`
	// Properties []PropertyAssign `json:"properties"`
}

type PropertyAssign struct {
	Definition *worktype.PropertyDefinition `json:"definition"`
	Value      string                       `json:"value"`
}
