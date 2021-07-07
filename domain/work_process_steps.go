package domain

import (
	"flywheel/domain/state"
	"time"

	"github.com/fundwit/go-commons/types"
)

type WorkProcessStep struct {
	WorkID        types.ID       `json:"workId"`
	FlowID        types.ID       `json:"flowId"`
	StateName     string         `json:"stateName"`
	StateCategory state.Category `json:"stateCategory"`

	CreatorID   types.ID        `json:"creatorId"`
	CreatorName string          `json:"creatorName"`
	BeginTime   types.Timestamp `json:"beginTime" sql:"type:DATETIME(6) NOT NULL"`

	EndTime types.Timestamp `json:"endTime" sql:"type:DATETIME(6)"`

	NextStateName     string         `json:"nextStateName"`
	NextStateCategory state.Category `json:"nextStateCategory"`
}

type WorkProcessStepCreation struct {
	FlowID    types.ID `json:"flowId" validate:"required"`
	WorkID    types.ID `json:"workId" validate:"required"`
	FromState string   `json:"fromState" validate:"required"`
	ToState   string   `json:"toState" validate:"required"`
}

type WorkProcessStepQuery struct {
	WorkID types.ID `json:"workId" form:"workId" binding:"required" validate:"required"`
}

// how long a work is existing:  now - work.createTime
// how long a work has been processed:
//    if the work's state is not stated:  sum(WorkProcessStep of this work)
//    if the work's state is WIP:         now - time of first enter WIP state
//    if the work's state is finished:    sum(WorkProcessStep of this work)

// how long a work is spend at one state
//    if the state is current state:      (now - time of last enter to the status) + sum(WorkProcessStep of this state)
//    if the state is not current state:  sum(WorkProcessStep of this state)
type WorkProcessStats struct {
	WorkID    types.ID
	FlowID    types.ID
	EnterTime types.ID
	LevelTime types.ID
}

type WorkProcessStateStats struct {
	WorkID         types.ID
	FlowID         types.ID
	StateName      string
	FirstEnterTime time.Time
	LastLeaveTime  time.Time
	TotalDuration  time.Duration
}
