package domain

import (
	"flywheel/utils"
	"time"
)

type Work struct {
	ID         utils.ID  `json:"id"`
	Name       string    `json:"name"`
	Group      string    `json:"group"`
	FlowID     utils.ID  `json:"flowId"`
	CreateTime time.Time `json:"createTime"`

	StateName string `json:"stateName"`
	// Properties []PropertyAssign `json:"properties"`
}

type PropertyDefinition struct {
	Name string `json:"name"`
}

type PropertyAssign struct {
	Definition *PropertyDefinition `json:"definition"`
	Value      string              `json:"value"`
}
