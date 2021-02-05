package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type WorkProcessStep struct {
	WorkID        types.ID       `json:"workId"`
	FlowID        types.ID       `json:"flowId"`
	StateName     string         `json:"stateName"`
	StateCategory state.Category `json:"stateCategory"`

	BeginTime time.Time  `json:"beginTime" sql:"type:DATETIME(3) NOT NULL"`
	EndTime   *time.Time `json:"endTime" sql:"type:DATETIME(3)"`
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
