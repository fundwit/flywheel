package flow

import (
	"github.com/fundwit/go-commons/types"
	"time"
)

type WorkStateTransition struct {
	ID         types.ID  `json:"id"`
	CreateTime time.Time `json:"createTime"`

	WorkStateTransitionBrief
}

type WorkStateTransitionBrief struct {
	FlowID    types.ID `json:"flowId" validate:"required"`
	WorkID    types.ID `json:"workId" validate:"required"`
	FromState string   `json:"fromState" validate:"required"`
	ToState   string   `json:"toState" validate:"required"`
}
