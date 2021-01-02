package domain

import (
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
	Type  WorkFlowBase `json:"type"`
	State state.State  `json:"state"`
}

func (c *WorkCreation) BuildWorkDetail(id utils.ID) *WorkDetail {
	workFlow := &GenericWorkFlow
	initState := GenericWorkFlow.StateMachine.States[0]

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
