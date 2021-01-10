package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type WorkCreation struct {
	Name    string   `json:"name" validate:"required"`
	GroupID types.ID `json:"groupId" validate:"required"`
}

type WorkUpdating struct {
	Name string `json:"name"`
}

type WorkDetail struct {
	Work
	Type  WorkFlowBase `json:"type"`
	State state.State  `json:"state"`
}

type WorkQuery struct {
	Name    string   `json:"name" form:"name"`
	GroupID types.ID `json:"groupId" form:"groupId"`
}

func (c *WorkCreation) BuildWorkDetail(id types.ID) *WorkDetail {
	workFlow := &GenericWorkFlow
	initState := GenericWorkFlow.StateMachine.States[0]

	return &WorkDetail{
		Work: Work{
			ID:      id,
			Name:    c.Name,
			GroupID: c.GroupID,
			FlowID:  workFlow.ID,

			StateName:  initState.Name,
			CreateTime: time.Now(),
		},
		Type:  workFlow.WorkFlowBase,
		State: initState,
	}
}
