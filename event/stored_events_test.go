package event

import (
	"flywheel/persistence"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var (
	testDatabase *testinfra.TestDatabase
)

func setup(t *testing.T) {
	testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
	assert.Nil(t, testDatabase.DS.GormDB().AutoMigrate(&EventRecord{}).Error)
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
			Synced:    true,
		}

		assert.Nil(t, eventPersistCreate(&event, testDatabase.DS.GormDB()))

		// assert records in tables
		records := []EventRecord{}
		Expect(testDatabase.DS.GormDB().Model(&EventRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(1))
		Expect(records[0]).To(Equal(event))
	})
}
