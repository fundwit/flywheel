package domain

import (
	"flywheel/domain/definition"
	"flywheel/domain/state"
	"time"
)

type Work struct {
	Type *definition.WorkType

	Name       string
	ID         uint64
	CreateTime time.Time

	State          state.State
	PropertyValues []PropertyAssign
}

type PropertyAssign struct {
	PropertyDefinition *definition.PropertyDefinition
	Value              string
}
