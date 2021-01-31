package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type WorkProcessStep struct {
	WorkID        types.ID
	FlowID        types.ID
	StateName     string
	StateCategory state.Category

	BeginTime *time.Time
	EndTime   *time.Time
}

// state transition record + state  record

// how long a work is existing:  now - work.createTime
// how long a work is been processed:
//    if the work's state is not stated:  0
//    if the work's state is WIP:         now - time of first enter WIP state
//    if the work's state is finished:    time of last enter to finished state  - time of first enter WIP state
// how long a work is spend at one state
//    if the state is not started:        0
//    if the state is current state:      (now - time of last enter to the status)
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
