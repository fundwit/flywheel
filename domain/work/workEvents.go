package work

import (
	"flywheel/app/event"
	"flywheel/domain"
	"flywheel/security"

	"github.com/jinzhu/gorm"
)

func CreateWorkCreatedEvent(w *domain.Work, identity *security.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryCreated, nil, nil, identity, db)
}
func CreateWorkDeletedEvent(w *domain.Work, identity *security.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryDeleted, nil, nil, identity, db)
}
func CreateWorkPropertyUpdatedEvent(w *domain.Work, updates []event.PropertyUpdated, identity *security.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryPropertyUpdated, updates, nil, identity, db)
}
func CreateWorkRelationUpdatedEvent(w *domain.Work, updates []event.RelationUpdated, identity *security.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryRelationUpdated, nil, updates, identity, db)
}
