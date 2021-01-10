package domain

import (
	"github.com/fundwit/go-commons/types"
	"time"
)

type Work struct {
	ID         types.ID  `json:"id"`
	Name       string    `json:"name"`
	GroupID    types.ID  `json:"groupId"`
	FlowID     types.ID  `json:"flowId"`
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
