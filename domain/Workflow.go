package domain

import (
	"flywheel/domain/state"
	"flywheel/utils"
)

type WorkFlowFace interface {
	FindState(string) *state.State
}

type WorkFlowBase struct {
	ID   utils.ID `json:"id"`
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

func FindWorkflow(ID utils.ID) *WorkFlow {
	if ID == GenericWorkFlow.ID {
		return &GenericWorkFlow
	}
	return nil
}

var GenericWorkFlow = WorkFlow{
	WorkFlowBase: WorkFlowBase{
		ID:   1,
		Name: "GenericTask",
	},
	StateMachine: *state.NewStateMachine([]state.State{{Name: "PENDING"}, {Name: "DOING"}, {Name: "DONE"}}, []state.Transition{
		{Name: "begin", From: state.State{Name: "PENDING"}, To: state.State{Name: "DOING"}},
		{Name: "close", From: state.State{Name: "PENDING"}, To: state.State{Name: "DONE"}},
		{Name: "cancel", From: state.State{Name: "DOING"}, To: state.State{Name: "PENDING"}},
		{Name: "finish", From: state.State{Name: "DOING"}, To: state.State{Name: "DONE"}},
		{Name: "reopen", From: state.State{Name: "DONE"}, To: state.State{Name: "PENDING"}},
	}),
	PropertyDefinitions: []PropertyDefinition{
		{Name: "description"}, {Name: "creatorId"},
	},
}
