package domain

import (
	"flywheel/domain/state"
	"time"
)

var StatePending = state.State{Name: "PENDING", Category: state.InBacklog}
var StateDoing = state.State{Name: "DOING", Category: state.InProcess}
var StateDone = state.State{Name: "DONE", Category: state.Done}

var GenericWorkflowTemplate = WorkflowTemplateDetail{
	WorkflowTemplate: WorkflowTemplate{
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
