package event

import (
	"flywheel/idgen"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	idWorker = sonyflake.NewSonyflake(sonyflake.Settings{})
)

func CreateEvent(sourceType string, sourceId types.ID, sourceDesc string, category EventCategory,
	updatedProperties []UpdatedProperty, updatedRelations []UpdatedRelation,
	identity *session.Identity, timestamp types.Timestamp, tx *gorm.DB) (*EventRecord, error) {

	record := EventRecord{
		ID: idgen.NextID(idWorker),
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
		Timestamp: timestamp,
	}

	if err := EventPersistCreateFunc(&record, tx); err != nil {
		return nil, err
	}
	return &record, nil
}
