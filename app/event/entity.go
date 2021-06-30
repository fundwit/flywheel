package event

import (
	"flywheel/common"

	"github.com/fundwit/go-commons/types"
)

const (
	EventCategoryCreated         = "CREATED"
	EventCategoryDeleted         = "DELETED"
	EventCategoryPropertyUpdated = "PROPERTY_UPDATED"
	EventCategoryRelationUpdated = "RELEATION_UPDATED"
)

type EventCategory string

type Event struct {
	SourceId   types.ID `json:"sourceId"`
	SourceType string   `json:"sourceType"`
	SourceDesc string   `json:"sourceTitle"`

	CreatorId   types.ID `json:"creatorId"`
	CreatorName string   `json:"creatorName"`

	EventCategory     EventCategory     `json:"eventCategory"` // CREATED, DELETED, PROPERTY_UPDATED, RELEATION_UPDATED
	UpdatedProperties []PropertyUpdated `json:"updatedProperties"`
	UpdatedRelations  []RelationUpdated `json:"updatedRelations"`
}

type EventRecord struct {
	Event

	Timestamp common.Timestamp `json:"timestamp"`
	Synced    bool             `json:"synced"`
}

func (r *EventRecord) TableName() string {
	return "events"
}

type PropertyUpdated struct {
	PropertyName string `json:"propertyName"`
	PropertyDesc string `json:"propertyDesc"`

	OldValue     string `json:"oldValue"`
	OldValueDesc string `json:"oldValueDesc"`
	NewValue     string `json:"newValue"`
	NewValueDesc string `json:"newValueDesc"`
}

type RelationUpdated struct {
	PropertyName string `json:"propertyName"`
	PropertyDesc string `json:"propertyDesc"`

	TargetType string `json:"targetType"`

	OldTargetId   string `json:"oldTargetId"`
	OldTargetDesc string `json:"oldTargetDesc"`
	NewTargetId   string `json:"newTargetId"`
	NewTargetDesc string `json:"newTargetDesc"`
}
