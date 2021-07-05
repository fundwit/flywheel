package work

import (
	"flywheel/domain"
	"flywheel/event"
	"flywheel/session"

	"github.com/jinzhu/gorm"
)

func CreateWorkCreatedEvent(w *domain.Work, identity *session.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryCreated, nil, nil, identity, db)
}
func CreateWorkDeletedEvent(w *domain.Work, identity *session.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryDeleted, nil, nil, identity, db)
}
func CreateWorkPropertyUpdatedEvent(w *domain.Work, updates []event.UpdatedProperty, identity *session.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryPropertyUpdated, updates, nil, identity, db)
}
func CreateWorkRelationUpdatedEvent(w *domain.Work, updates []event.UpdatedRelation, identity *session.Identity, db *gorm.DB) error {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryRelationUpdated, nil, updates, identity, db)
}
