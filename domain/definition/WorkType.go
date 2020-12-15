package definition

import (
	"flywheel/domain/state"
)

type WorkType struct {
	ID   string
	Name string

	PropertyDefinitions []PropertyDefinition
	StateMachine        state.StateMachine
}

type PropertyDefinition struct {
	Name string
}

var GenericWorkType = WorkType{
	ID:   "1",
	Name: "GenericTask",
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
