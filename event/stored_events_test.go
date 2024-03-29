package event

import (
	"context"
	"flywheel/indices/indexlog"
	"flywheel/persistence"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var (
	testDatabase *testinfra.TestDatabase
)

func setup(t *testing.T) {
	testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
	assert.Nil(t, testDatabase.DS.GormDB(context.Background()).AutoMigrate(&EventRecord{}).Error)
	persistence.ActiveDataSourceManager = testDatabase.DS
}
func teardown(t *testing.T) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestEventPersistCreate(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to persist event create", func(t *testing.T) {
		setup(t)
		defer teardown(t)

		indexlog.CreateIndexLogFunc = func(id types.ID, sourceType string, sourceId types.ID, sourceDesc string,
			deletion bool, timestamp types.Timestamp, tx *gorm.DB) (*indexlog.IndexLogRecord, error) {
			return nil, nil
		}

		event := EventRecord{
			Event: Event{
				SourceType: "WORK",
				SourceId:   1234,
				SourceDesc: "work1234",

				EventCategory: EventCategoryCreated,
				UpdatedProperties: UpdatedProperties{{PropertyName: "Name", PropertyDesc: "NameDesc",
					OldValue: "OldName", OldValueDesc: "OldNameDesc", NewValue: "NewName", NewValueDesc: "NewNameDesc"}},
				UpdatedRelations: UpdatedRelations{{PropertyName: "Address", PropertyDesc: "AddressDesc",
					TargetType: "address", TargetTypeDesc: "Address",
					OldTargetId: "addressOld", OldTargetDesc: "addressOldDesc", NewTargetId: "addressNew", NewTargetDesc: "addressNewDesc"}},

				CreatorId:   333,
				CreatorName: "user333",
			},
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}

		assert.Nil(t, eventPersistCreate(&event, testDatabase.DS.GormDB(context.Background())))

		// assert records in tables
		records := []EventRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&EventRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(1))
		Expect(records[0]).To(Equal(event))
	})
}
