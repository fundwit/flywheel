package domain

import (
	"flywheel/domain/state"
	"flywheel/domain/worktype"
	"flywheel/utils"
	"time"
)

type WorkCreation struct {
	Name  string `json:"name" validate:"required"`
	Group string `json:"group" validate:"required"`
}

type WorkDetail struct {
	Work
	Type  worktype.WorkTypeBase `json:"type"`
	State state.State           `json:"state"`
}

func (c *WorkCreation) BuildWorkDetail(id utils.ID) *WorkDetail {
	workType := &worktype.GenericWorkType
	initState := worktype.GenericWorkType.StateMachine.States[0]

	return &WorkDetail{
		Work: Work{
			ID:     id,
			Name:   c.Name,
			Group:  c.Group,
			TypeID: workType.ID,

			StateName:  initState.Name,
			CreateTime: time.Now(),
		},
		Type:  workType.WorkTypeBase,
		State: initState,
	}
}
