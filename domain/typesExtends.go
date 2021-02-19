package domain

import (
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

type StageRangeOrderUpdating struct {
	ID       types.ID `json:"id" validate:"required"`
	NewOlder int64    `json:"newOrder"`
	OldOlder int64    `json:"oldOrder"`
}

type WorkDetail struct {
	Work
	Type Workflow `json:"type"`
}

type WorkQuery struct {
	Name    string   `json:"name" form:"name"`
	GroupID types.ID `json:"groupId" form:"groupId"`
}

type GroupRole struct {
	GroupID   types.ID `json:"groupId"`
	GroupName string   `json:"groupName"`
	Role      string   `json:"role"`
}

func (c *WorkCreation) BuildWorkDetail(id types.ID) *WorkDetail {
	workFlow := &GenericWorkFlow
	initState := GenericWorkFlow.StateMachine.States[0]

	now := time.Now()
	return &WorkDetail{
		Work: Work{
			ID:         id,
			Name:       c.Name,
			GroupID:    c.GroupID,
			CreateTime: now,

			FlowID:         workFlow.ID,
			OrderInState:   now.UnixNano() / 1e6,
			StateName:      initState.Name,
			StateBeginTime: &now,
			State:          initState,
		},
		Type: workFlow.Workflow,
	}
}
