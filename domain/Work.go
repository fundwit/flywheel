package domain

import (
	"flywheel/domain/flow"
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

type PropertyAssign struct {
	Definition *flow.PropertyDefinition `json:"definition"`
	Value      string                   `json:"value"`
}
