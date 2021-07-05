package event

import (
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

func CreateEvent(sourceType string, sourceId types.ID, sourceDesc string, category EventCategory,
	updatedProperties []UpdatedProperty, updatedRelations []UpdatedRelation,
	identity *session.Identity, db *gorm.DB) error {

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
		Timestamp: types.CurrentTimestamp(),
	}
	return EventPersistCreateFunc(&record, db)
}
