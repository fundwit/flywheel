package flow

import (
	"flywheel/utils"
	"time"
)

type WorkStateTransition struct {
	ID         utils.ID  `json:"id"`
	CreateTime time.Time `json:"createTime"`

	WorkStateTransitionBrief
}

type WorkStateTransitionBrief struct {
	FlowID    utils.ID `json:"flowId" validate:"required"`
	WorkID    utils.ID `json:"workId" validate:"required"`
	FromState string   `json:"fromState" validate:"required"`
	ToState   string   `json:"toState" validate:"required"`
}
