package flow

import (
	"flywheel/domain/state"

	"github.com/fundwit/go-commons/types"
)

type WorkflowCreation struct {
	Name       string   `json:"name"       binding:"required"`
	ProjectID  types.ID `json:"projectId"    binding:"required"`
	ThemeColor string   `json:"themeColor" binding:"required"`
	ThemeIcon  string   `json:"themeIcon"  binding:"required"`

	StateMachine state.StateMachine `json:"stateMachine" binding:"dive"`
}

type WorkflowBaseUpdation struct {
	Name       string `json:"name"     binding:"required"`
	ThemeColor string `json:"themeColor" binding:"required"`
	ThemeIcon  string `json:"themeIcon"  binding:"required"`
}

type WorkflowStateUpdating struct {
	OriginName string `json:"originName"  binding:"required"`

	Name  string `json:"name"        binding:"required"`
	Order int    `json:"order"`
}

type StateOrderRangeUpdating struct {
	State    string `json:"state" validate:"required"`
	NewOlder int    `json:"newOrder"`
	OldOlder int    `json:"oldOrder"`
}
type StateCreating struct {
	Name        string             `json:"name"         binding:"required"`
	Category    state.Category     `json:"category"     binding:"required"`
	Order       int                `json:"order"        binding:"required"`
	Transitions []state.Transition `json:"transitions"  binding:"dive"`
}
