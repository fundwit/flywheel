package event

import "github.com/jinzhu/gorm"

var (
	EventPersistCreateFunc = eventPersistCreate
)

func eventPersistCreate(record *EventRecord, tx *gorm.DB) error {
	return tx.Create(record).Error
}
