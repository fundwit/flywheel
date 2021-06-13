package domain

import (
	"flywheel/domain/state"
	"time"

	"github.com/fundwit/go-commons/types"
)

type WorkflowTemplateDetail struct {
	WorkflowTemplate

	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
	StateMachine        state.StateMachine   `json:"stateMachine"`
}

type WorkflowTemplate struct {
	ID   types.ID `json:"id" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL"`
	Name string   `json:"name"`

	ProjectID  types.ID  `json:"projectId"`
	CreateTime time.Time `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`
}
