package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type Workflow struct {
	ID         types.ID `json:"id" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Name       string   `json:"name"`
	ThemeColor string   `json:"themeColor"`
	ThemeIcon  string   `json:"themeIcon"`

	GroupID    types.ID  `json:"groupId"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

type WorkflowDetail struct {
	Workflow

	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
	StateMachine        state.StateMachine   `json:"stateMachine"`
}

type WorkflowState struct {
	WorkflowID types.ID `json:"workflowId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Name       string   `json:"name" gorm:"primary_key"`
	Order      int      `json:"order"`

	Category   state.Category `json:"category"`
	CreateTime time.Time      `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

type WorkflowStateTransition struct {
	WorkflowID types.ID `json:"workflowId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	FromState  string   `json:"fromState" gorm:"primary_key"`
	ToState    string   `json:"toState"     gorm:"primary_key"`

	Name       string    `json:"name"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}

func (f *WorkflowDetail) FindState(stateName string) (state.State, bool) {
	return f.StateMachine.FindState(stateName)
}
