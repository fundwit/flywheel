package event

import (
	"flywheel/common"
	"flywheel/security"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

var (
	EventPersistCreateFunc = eventPersistCreate
)

func CreateEvent(sourceType string, sourceId types.ID, sourceDesc string, category EventCategory,
	updatedProperties []PropertyUpdated, updatedRelations []RelationUpdated,
	identity *security.Identity, db *gorm.DB) error {

	record := EventRecord{
		Event: Event{
			SourceType: sourceType,
			SourceId:   sourceId,
			SourceDesc: sourceDesc,

			EventCategory:     category,
			UpdatedProperties: updatedProperties,
			UpdatedRelations:  updatedRelations,

			CreatorId:   identity.ID,
			CreatorName: identity.Name,
		},
		Synced:    false,
		Timestamp: common.CurrentTimestamp(),
	}
	return EventPersistCreateFunc(&record, db)
}

func eventPersistCreate(record *EventRecord, db *gorm.DB) error {
	return db.Create(record).Error
}
