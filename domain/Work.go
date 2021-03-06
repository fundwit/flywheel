package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type Work struct {
	ID         types.ID  `json:"id"`
	Name       string    `json:"name"`
	GroupID    types.ID  `json:"groupId"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`

	FlowID types.ID `json:"flowId"`

	// bigger OrderInState means lower priority
	// max integer number in javascript is:        9007199254740991 (2^53-1)
	// Unix millisecond of 9999-12-31 23:59:59 is: 253402271999000  (safe for javascript)
	OrderInState     int64          `json:"orderInState"`
	StateName        string         `json:"stateName"`
	StateCategory    state.Category `json:"stateCategory"`
	State            state.State    `json:"state"`
	StateBeginTime   *time.Time     `json:"stateBeginTime" sql:"type:DATETIME(3)"`
	ProcessBeginTime *time.Time     `json:"processBeginTime" sql:"type:DATETIME(3)"`
	ProcessEndTime   *time.Time     `json:"processEndTime" sql:"type:DATETIME(3)"`

	// Properties []PropertyAssign `json:"properties"`
}

type PropertyDefinition struct {
	Name string `json:"name"`
}

type PropertyAssign struct {
	Definition *PropertyDefinition `json:"definition"`
	Value      string              `json:"value"`
}
