package event

import "github.com/jinzhu/gorm"

var (
	EventPersistCreateFunc = eventPersistCreate
)

func eventPersistCreate(record *EventRecord, db *gorm.DB) error {
	return db.LogMode(true).Create(record).Error
}
