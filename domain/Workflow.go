package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
)

type WorkFlowBase struct {
	ID   types.ID `json:"id"`
	Name string   `json:"name"`
}

type WorkFlow struct {
	WorkFlowBase

	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
	StateMachine        state.StateMachine   `json:"stateMachine"`
}

func (wt *WorkFlow) FindState(stateName string) (state.State, bool) {
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

var GenericWorkFlow = WorkFlow{
	WorkFlowBase: WorkFlowBase{
		ID:   1,
		Name: "GenericTask",
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
