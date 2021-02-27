package domain

import (
	"github.com/fundwit/go-commons/types"
)

type WorkCreation struct {
	Name    string   `json:"name" validate:"required"`
	GroupID types.ID `json:"groupId" validate:"required"`
	FlowID  types.ID `json:"flowId" validate:"required"`
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

type WorkflowQuery struct {
	Name    string   `json:"name" form:"name"`
	GroupID types.ID `json:"groupId" form:"groupId"`
}

type GroupRole struct {
	GroupID   types.ID `json:"groupId"`
	GroupName string   `json:"groupName"`
	Role      string   `json:"role"`
}
