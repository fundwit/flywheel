package worktype

import (
	"errors"
	"flywheel/domain/state"
	"flywheel/utils"
)

type WorkTypeFace interface {
	FindState(string) *state.State
}

type WorkTypeBase struct {
	ID   utils.ID `json:"id"`
	Name string   `json:"name"`
}

type WorkType struct {
	WorkTypeBase

	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
	StateMachine        state.StateMachine   `json:"stateMachine"`
}

type PropertyDefinition struct {
	Name string `json:"name"`
}

func (wt *WorkType) FindState(stateName string) (state.State, error) {
	for _, s := range wt.StateMachine.States {
		if stateName == s.Name {
			return s, nil
		}
	}
	return state.State{}, errors.New("invalid state name")
}

var GenericWorkType = WorkType{
	WorkTypeBase: WorkTypeBase{
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
