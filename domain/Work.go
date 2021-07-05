package domain

import (
	"flywheel/domain/state"

	"github.com/fundwit/go-commons/types"
)

type Work struct {
	ID         types.ID        `json:"id" gorm:"primary_key"`
	Identifier string          `json:"identifier" gorm:"unique_index:identifier_unique"`
	Name       string          `json:"name"`
	ProjectID  types.ID        `json:"projectId"`
	CreateTime types.Timestamp `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`

	FlowID types.ID `json:"flowId"`

	// bigger OrderInState means lower priority
	// max integer number in javascript is:        9007199254740991 (2^53-1)
	// Unix millisecond of 9999-12-31 23:59:59 is: 253402271999000  (safe for javascript)
	OrderInState     int64           `json:"orderInState"`
	StateName        string          `json:"stateName"`
	StateCategory    state.Category  `json:"stateCategory"`
	State            state.State     `json:"state"`
	StateBeginTime   types.Timestamp `json:"stateBeginTime" sql:"type:DATETIME(6)"`
	ProcessBeginTime types.Timestamp `json:"processBeginTime" sql:"type:DATETIME(6)"`
	ProcessEndTime   types.Timestamp `json:"processEndTime" sql:"type:DATETIME(6)"`

	// Properties []PropertyAssign `json:"properties"`

	ArchiveTime types.Timestamp `json:"archivedTime" sql:"type:DATETIME(6)"`
}

type PropertyDefinition struct {
	Name string `json:"name"`
}

type PropertyAssign struct {
	Definition *PropertyDefinition `json:"definition"`
	Value      string              `json:"value"`
}
