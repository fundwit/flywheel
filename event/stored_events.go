package event

import (
	"flywheel/indices/indexlog"

	"github.com/jinzhu/gorm"
)

var (
	EventPersistCreateFunc = eventPersistCreate
)

func eventPersistCreate(record *EventRecord, tx *gorm.DB) error {
	_, err := indexlog.CreateIndexLogFunc(record.ID, record.SourceType, record.SourceId, record.SourceDesc,
		record.EventCategory == EventCategoryDeleted, record.Timestamp, tx)
	if err != nil {
		return err
	}

	return tx.Create(record).Error
}
