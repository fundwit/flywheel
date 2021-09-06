package domain

import (
	"flywheel/domain/state"

	"github.com/fundwit/go-commons/types"
)

type WorkCreation struct {
	Name      string   `json:"name" binding:"required"`
	ProjectID types.ID `json:"projectId" binding:"required"`
	FlowID    types.ID `json:"flowId" binding:"required"`

	InitialStateName string `json:"initialStateName" binding:"required"`
	PriorityLevel    int    `json:"priorityLevel"`
}

type WorkUpdating struct {
	Name string `json:"name"`
}

type WorkOrderRangeUpdating struct {
	ID       types.ID `json:"id" binding:"required"`
	NewOlder int64    `json:"newOrder"`
	OldOlder int64    `json:"oldOrder"`
}

const (
	StatusOn  = "ON"
	StatusOff = "OFF" // default
	StatusAll = "ALL"
)

type WorkQuery struct {
	Name            string           `json:"name" form:"name"`
	ProjectID       types.ID         `json:"projectId" form:"projectId"`
	StateCategories []state.Category `json:"stateCategories" form:"stateCategory"`

	ArchiveState string `json:"archiveState" form:"archiveState" binding:"omitempty,oneof=ON OFF ALL"`
}

type WorkSelection struct {
	WorkIdList []types.ID `json:"workIdList" form:"workIdList" binding:"required,gt=0"`
}

type WorkflowQuery struct {
	Name      string   `json:"name" form:"name"`
	ProjectID types.ID `json:"projectId" form:"projectId"`
}

type ProjectRole struct {
	ProjectID types.ID `json:"projectId"`
	Role      string   `json:"role"`

	ProjectName       string `json:"projectName"`
	ProjectIdentifier string `json:"projectIdentifier"`
}
