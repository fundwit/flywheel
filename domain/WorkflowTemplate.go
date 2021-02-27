package domain

import (
	"flywheel/domain/state"
	"github.com/fundwit/go-commons/types"
	"time"
)

type WorkflowTemplateDetail struct {
	WorkflowTemplate

	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
	StateMachine        state.StateMachine   `json:"stateMachine"`
}

type WorkflowTemplate struct {
	ID   types.ID `json:"id" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Name string   `json:"name"`

	GroupID    types.ID  `json:"groupId"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(3) NOT NULL"`
}
