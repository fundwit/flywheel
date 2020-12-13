package domain

import (
	"flywheel/domain/definition"
	"time"
)

type WorkManager struct {
}

func (m *WorkManager) Create(c *WorkCreation) *Work {
	workType := &definition.GenericWorkType
	initState := definition.GenericWorkType.StateMachine.States[0]

	ID := uint64(123)

	workItem := &Work{
		Type:       workType,
		Name:       c.Name,
		ID:         ID,
		State:      initState,
		CreateTime: time.Now(),
	}

	// TODO save workItem

	return workItem
}
