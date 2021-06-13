package domain

import (
	"flywheel/common"

	"github.com/fundwit/go-commons/types"
)

type WorkStateTransition struct {
	ID         types.ID         `json:"id"`
	CreateTime common.Timestamp `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`
	Creator    types.ID         `json:"creator"`

	WorkStateTransitionBrief
}

type WorkStateTransitionBrief struct {
	FlowID    types.ID `json:"flowId" validate:"required"`
	WorkID    types.ID `json:"workId" validate:"required"`
	FromState string   `json:"fromState" validate:"required"`
	ToState   string   `json:"toState" validate:"required"`
}
