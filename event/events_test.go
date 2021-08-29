package event_test

import (
	"errors"
	"flywheel/event"
	"flywheel/session"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func TestCreateEvent(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should return error when failed to persist event", func(t *testing.T) {
		testErr := errors.New("test error")
		event.EventPersistCreateFunc = func(record *event.EventRecord, tx *gorm.DB) error {
			return testErr
		}
		var tx = &gorm.DB{Value: 10000}
		ret, err := event.CreateEvent("WORK", 1234, "work1234", event.EventCategoryCreated,
			event.UpdatedProperties{{PropertyName: "Name", PropertyDesc: "NameDesc",
				OldValue: "OldName", OldValueDesc: "OldNameDesc", NewValue: "NewName", NewValueDesc: "NewNameDesc"}},
			event.UpdatedRelations{{PropertyName: "Address", PropertyDesc: "AddressDesc",
				TargetType: "address", TargetTypeDesc: "Address",
				OldTargetId: "addressOld", OldTargetDesc: "addressOldDesc", NewTargetId: "addressNew", NewTargetDesc: "addressNewDesc"}},
			&session.Identity{ID: 333, Name: "user333"},
			types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			tx,
		)
		Expect(ret).To(BeNil())
		Expect(err).To(Equal(testErr))
	})

	t.Run("should be able to create events", func(t *testing.T) {
		var ev event.EventRecord
		var db *gorm.DB
		event.EventPersistCreateFunc = func(record *event.EventRecord, tx *gorm.DB) error {
			ev = *record
			db = tx
			return nil
		}

		var tx = &gorm.DB{Value: 10000}
		ret, err := event.CreateEvent("WORK", 1234, "work1234", event.EventCategoryCreated,
			event.UpdatedProperties{{PropertyName: "Name", PropertyDesc: "NameDesc",
				OldValue: "OldName", OldValueDesc: "OldNameDesc", NewValue: "NewName", NewValueDesc: "NewNameDesc"}},
			event.UpdatedRelations{{PropertyName: "Address", PropertyDesc: "AddressDesc",
				TargetType: "address", TargetTypeDesc: "Address",
				OldTargetId: "addressOld", OldTargetDesc: "addressOldDesc", NewTargetId: "addressNew", NewTargetDesc: "addressNewDesc"}},
			&session.Identity{ID: 333, Name: "user333"},
			types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			tx,
		)
		Expect(err).To(BeNil())

		expectEvent := event.EventRecord{
			Event: event.Event{
				SourceType: "WORK",
				SourceId:   1234,
				SourceDesc: "work1234",

				EventCategory: event.EventCategoryCreated,
				UpdatedProperties: event.UpdatedProperties{{PropertyName: "Name", PropertyDesc: "NameDesc",
					OldValue: "OldName", OldValueDesc: "OldNameDesc", NewValue: "NewName", NewValueDesc: "NewNameDesc"}},
				UpdatedRelations: event.UpdatedRelations{{PropertyName: "Address", PropertyDesc: "AddressDesc",
					TargetType: "address", TargetTypeDesc: "Address",
					OldTargetId: "addressOld", OldTargetDesc: "addressOldDesc", NewTargetId: "addressNew", NewTargetDesc: "addressNewDesc"}},

				CreatorId:   333,
				CreatorName: "user333",
			},
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			Synced:    false,
		}

		Expect(*ret).To(Equal(expectEvent))
		Expect(ev).To(Equal(expectEvent))

		Expect(db).To(Equal(tx))
	})
}
