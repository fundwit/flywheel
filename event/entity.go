package event

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/fundwit/go-commons/types"
)

const (
	EventCategoryCreated          = "CREATED"
	EventCategoryDeleted          = "DELETED"
	EventCategoryPropertyUpdated  = "PROPERTY_UPDATED"
	EventCategoryRelationUpdated  = "RELATION_UPDATED"
	EventCategoryExtensionUpdated = "EXTENSION_UPDATED"
)

type EventCategory string

type Event struct {
	SourceId   types.ID `json:"sourceId"`
	SourceType string   `json:"sourceType"`
	SourceDesc string   `json:"sourceDesc"`

	CreatorId   types.ID `json:"creatorId"`
	CreatorName string   `json:"creatorName"`

	EventCategory     EventCategory     `json:"eventCategory"` // CREATED, DELETED, PROPERTY_UPDATED, RELEATION_UPDATED
	UpdatedProperties UpdatedProperties `json:"updatedProperties" sql:"type:TEXT"`
	UpdatedRelations  UpdatedRelations  `json:"updatedRelations" sql:"type:TEXT"`
}

type EventRecord struct {
	Event

	Timestamp types.Timestamp `json:"timestamp" sql:"type:DATETIME(6)"`
	Synced    bool            `json:"synced"`
}

func (r *EventRecord) TableName() string {
	return "events"
}

type UpdatedProperty struct {
	PropertyName string `json:"propertyName"`
	PropertyDesc string `json:"propertyDesc"`

	OldValue     string `json:"oldValue"`
	OldValueDesc string `json:"oldValueDesc"`
	NewValue     string `json:"newValue"`
	NewValueDesc string `json:"newValueDesc"`
}

type UpdatedExtension struct {
	PropertyName string `json:"propertyName"`
	PropertyDesc string `json:"propertyDesc"`

	OldValue     string `json:"oldValue"`
	OldValueDesc string `json:"oldValueDesc"`
	NewValue     string `json:"newValue"`
	NewValueDesc string `json:"newValueDesc"`
}

type UpdatedProperties []UpdatedProperty

type UpdatedRelation struct {
	PropertyName string `json:"propertyName"`
	PropertyDesc string `json:"propertyDesc"`

	TargetType     string `json:"targetType"`
	TargetTypeDesc string `json:"targetTypeDesc"`

	OldTargetId   string `json:"oldTargetId"`
	OldTargetDesc string `json:"oldTargetDesc"`
	NewTargetId   string `json:"newTargetId"`
	NewTargetDesc string `json:"newTargetDesc"`
}

type UpdatedRelations []UpdatedRelation

func (t UpdatedProperties) Value() (driver.Value, error) {
	jsonBytes, err := json.Marshal(&t)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

func (c *UpdatedProperties) Scan(v interface{}) error {
	jsonString, ok := v.(string)
	if !ok {
		jsonByte, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("type is neither string nor []byte: %T %v", v, v)
		}
		jsonString = string(jsonByte)
	}
	return json.Unmarshal([]byte(jsonString), c)
}

func (t UpdatedRelations) Value() (driver.Value, error) {
	jsonBytes, err := json.Marshal(&t)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

func (c *UpdatedRelations) Scan(v interface{}) error {
	jsonString, ok := v.(string)
	if !ok {
		jsonByte, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("type is neither string nor []byte: %T %v", v, v)
		}
		jsonString = string(jsonByte)
	}
	return json.Unmarshal([]byte(jsonString), c)
}
