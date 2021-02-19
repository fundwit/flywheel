package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type Workflow struct {
	ID   types.ID `json:"id" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Name string   `json:"name"`

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

func (wt *WorkflowDetail) FindState(stateName string) (state.State, bool) {
	for _, s := range wt.StateMachine.States {
		if stateName == s.Name {
			return s, true
		}
	}
	return state.State{}, false
}

var StatePending = state.State{Name: "PENDING", Category: state.InBacklog}
var StateDoing = state.State{Name: "DOING", Category: state.InProcess}
var StateDone = state.State{Name: "DONE", Category: state.Done}

var GenericWorkFlow = WorkflowDetail{
	Workflow: Workflow{
		ID:         1,
		Name:       "GenericTask",
		GroupID:    0,
		CreateTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	StateMachine: *state.NewStateMachine([]state.State{StatePending, StateDoing, StateDone}, []state.Transition{
		{Name: "begin", From: StatePending, To: StateDoing},
		{Name: "close", From: StatePending, To: StateDone},
		{Name: "cancel", From: StateDoing, To: StatePending},
		{Name: "finish", From: StateDoing, To: StateDone},
		{Name: "reopen", From: StateDone, To: StatePending},
	}),
	PropertyDefinitions: []PropertyDefinition{
		{Name: "description"}, {Name: "creatorId"},
	},
}
