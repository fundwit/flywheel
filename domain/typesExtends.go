package domain

import (
	"flywheel/domain/flow"
	"flywheel/domain/state"
	"flywheel/utils"
	"time"
)

type WorkCreation struct {
	Name  string `json:"name" validate:"required"`
	Group string `json:"group" validate:"required"`
}

type WorkDetail struct {
	Work
	Type  flow.WorkFlowBase `json:"type"`
	State state.State       `json:"state"`
}

func (c *WorkCreation) BuildWorkDetail(id utils.ID) *WorkDetail {
	workFlow := &flow.GenericWorkFlow
	initState := flow.GenericWorkFlow.StateMachine.States[0]

	return &WorkDetail{
		Work: Work{
			ID:     id,
			Name:   c.Name,
			Group:  c.Group,
			FlowID: workFlow.ID,

			StateName:  initState.Name,
			CreateTime: time.Now(),
		},
		Type:  workFlow.WorkFlowBase,
		State: initState,
	}
}
