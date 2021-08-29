package work

import (
	"flywheel/domain"
	"flywheel/event"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

func CreateWorkCreatedEvent(w *domain.Work, identity *session.Identity, timestamp types.Timestamp, db *gorm.DB) (*event.EventRecord, error) {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryCreated, nil, nil, identity, timestamp, db)
}
func CreateWorkDeletedEvent(w *domain.Work, identity *session.Identity, timestamp types.Timestamp, db *gorm.DB) (*event.EventRecord, error) {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryDeleted, nil, nil, identity, timestamp, db)
}
func CreateWorkPropertyUpdatedEvent(w *domain.Work, updates []event.UpdatedProperty, identity *session.Identity, timestamp types.Timestamp, db *gorm.DB) (*event.EventRecord, error) {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryPropertyUpdated, updates, nil, identity, timestamp, db)
}
func CreateWorkRelationUpdatedEvent(w *domain.Work, updates []event.UpdatedRelation, identity *session.Identity, timestamp types.Timestamp, db *gorm.DB) (*event.EventRecord, error) {
	return event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryRelationUpdated, nil, updates, identity, timestamp, db)
}
