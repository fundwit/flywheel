package domain

import (
	"flywheel/domain/state"
	"time"
)

var StatePending = state.State{Name: "PENDING", Category: state.InBacklog, Order: 1}
var StateDoing = state.State{Name: "DOING", Category: state.InProcess, Order: 2}
var StateDone = state.State{Name: "DONE", Category: state.Done, Order: 3}

var GenericWorkflowTemplate = WorkflowTemplateDetail{
	WorkflowTemplate: WorkflowTemplate{
		ID:         1,
		Name:       "GenericTask",
		ProjectID:  0,
		CreateTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	StateMachine: *state.NewStateMachine([]state.State{StatePending, StateDoing, StateDone}, []state.Transition{
		{Name: "begin", From: StatePending.Name, To: StateDoing.Name},
		{Name: "close", From: StatePending.Name, To: StateDone.Name},
		{Name: "cancel", From: StateDoing.Name, To: StatePending.Name},
		{Name: "finish", From: StateDoing.Name, To: StateDone.Name},
		{Name: "reopen", From: StateDone.Name, To: StatePending.Name},
	}),
	PropertyDefinitions: []PropertyDefinition{
		{Name: "description"}, {Name: "creatorId"},
	},
}
