package indexlog

import (
	"context"
	"errors"
	"flywheel/persistence"
	"flywheel/testinfra"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

func TestCreateIndexLog(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should return error when failed to persist index log", func(t *testing.T) {
		testErr := errors.New("test error")
		IndexLogPersistCreateFunc = func(record *IndexLogRecord, tx *gorm.DB) error {
			return testErr
		}
		var tx = &gorm.DB{Value: 10000}
		ret, err := CreateIndexLog(100, "WORK", 1234, "work1234", true,
			types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			tx,
		)
		Expect(ret).To(BeNil())
		Expect(err).To(Equal(testErr))
	})

	t.Run("should be able to create index log", func(t *testing.T) {
		var log IndexLogRecord
		var db *gorm.DB
		IndexLogPersistCreateFunc = func(record *IndexLogRecord, tx *gorm.DB) error {
			log = *record
			db = tx
			return nil
		}

		var tx = &gorm.DB{Value: 10000}
		ret, err := CreateIndexLog(100, "WORK", 1234, "work1234", true,
			types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			tx,
		)
		Expect(err).To(BeNil())

		expectIndexLog := IndexLogRecord{
			IndexLog: IndexLog{
				SourceType: "WORK",
				SourceId:   1234,
				SourceDesc: "work1234",
				Deletion:   true,
			},
			ID:          100,
			Timestamp:   types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			IndexedTime: types.Timestamp{},
		}
		Expect(*ret).To(Equal(expectIndexLog))
		Expect(log).To(Equal(expectIndexLog))
		Expect(db).To(Equal(tx))
	})
}

func indexLogPersistTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) {
	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	Expect(db.DS.GormDB(context.Background()).AutoMigrate(&IndexLogRecord{}).Error).To(BeNil())
	persistence.ActiveDataSourceManager = db.DS
}
func indexLogPersistTestTeardownTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestIndexLogPersistCreate(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to persist event create", func(t *testing.T) {
		defer indexLogPersistTestTeardownTeardown(t, testDatabase)
		indexLogPersistTestSetup(t, &testDatabase)

		indexlog1 := IndexLogRecord{
			IndexLog:    IndexLog{SourceType: "WORK", SourceId: 1000, SourceDesc: "work1000", Deletion: false},
			ID:          100,
			Timestamp:   types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			IndexedTime: types.TimestampOfDate(2021, 1, 1, 12, 12, 13, 0, time.Local),
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog1, testDatabase.DS.GormDB(context.Background())))
		// assert records in tables
		records := []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(1))
		Expect(records[0]).To(Equal(indexlog1))

		indexlog1a := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 1000, SourceDesc: "work1000", Deletion: false},
			ID:        110,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog1a, testDatabase.DS.GormDB(context.Background())))
		records = []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(2))
		Expect(records[1]).To(Equal(indexlog1a))

		// insert data2
		indexlog2 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 2000, SourceDesc: "work2000", Deletion: true},
			ID:        200,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog2, testDatabase.DS.GormDB(context.Background())))
		// assert records in tables
		records = []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(3))
		Expect(records[2]).To(Equal(indexlog2))

		// insert data3
		indexlog1b := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 1000, SourceDesc: "work1000", Deletion: true},
			ID:        300,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog1b, testDatabase.DS.GormDB(context.Background())))
		// assert records in tables
		records = []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(len(records)).To(Equal(4))
		Expect(records[3]).To(Equal(indexlog1b))
		Expect(records[2]).To(Equal(indexlog2)) // indexlog2 not changed cause of work id not match
		indexlog1a.Obsolete = true
		Expect(records[1]).To(Equal(indexlog1a)) // indexlog1a has be obsoleted
		Expect(records[0]).To(Equal(indexlog1))  // indexlog1 not changed cause of it already be indexed
	})
}

func TestFinishIndexLog(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to finish index log", func(t *testing.T) {
		defer indexLogPersistTestTeardownTeardown(t, testDatabase)
		indexLogPersistTestSetup(t, &testDatabase)

		indexlog1 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 1000, SourceDesc: "work1000", Deletion: false},
			ID:        110,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			Obsolete:  true,
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog1, testDatabase.DS.GormDB(context.Background())))
		Expect(FinishIndexLog(indexlog1.ID)).To(BeNil())
		records := []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(time.Since(records[0].IndexedTime.Time()) < time.Second).To(BeTrue())
		Expect(records[0].Obsolete).To(BeFalse())

		// indexed record still be able to updated indexed time
		oldIndexedTime := records[0].IndexedTime
		time.Sleep(10 * time.Millisecond)
		Expect(FinishIndexLog(indexlog1.ID)).To(BeNil())
		records = []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(records[0].IndexedTime.Time().After(oldIndexedTime.Time())).To(BeTrue())
		Expect(records[0].Obsolete).To(BeFalse())
	})
}

func TestObsoleteIndexLog(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to obsolete index log", func(t *testing.T) {
		defer indexLogPersistTestTeardownTeardown(t, testDatabase)
		indexLogPersistTestSetup(t, &testDatabase)

		indexlog1 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 1000, SourceDesc: "work1000", Deletion: false},
			ID:        110,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		assert.Nil(t, indexLogPersistCreate(&indexlog1, testDatabase.DS.GormDB(context.Background())))
		Expect(ObsoleteIndexLog(indexlog1.ID)).To(BeNil())
		records := []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(records[0].IndexedTime).To(BeZero())
		Expect(records[0].Obsolete).To(BeTrue())

		// obsoleted record still be able to updated indexed time
		time.Sleep(10 * time.Millisecond)
		Expect(ObsoleteIndexLog(indexlog1.ID)).To(BeNil())
		records = []IndexLogRecord{}
		Expect(testDatabase.DS.GormDB(context.Background()).Model(&IndexLogRecord{}).Find(&records).Error).To(BeNil())
		Expect(records[0].IndexedTime).To(BeZero())
		Expect(records[0].Obsolete).To(BeTrue())
	})
}

func TestLoadPendingIndexLog(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should be able to load pendinged index log", func(t *testing.T) {
		defer indexLogPersistTestTeardownTeardown(t, testDatabase)
		indexLogPersistTestSetup(t, &testDatabase)

		indexlog1 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 10001, SourceDesc: "work10001", Deletion: false},
			ID:        101,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		Expect(indexLogPersistCreate(&indexlog1, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		indexlog2 := IndexLogRecord{
			IndexLog:    IndexLog{SourceType: "WORK", SourceId: 10002, SourceDesc: "work10002", Deletion: false},
			ID:          102,
			Timestamp:   types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			IndexedTime: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		Expect(indexLogPersistCreate(&indexlog2, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		indexlog3 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 10003, SourceDesc: "work10003", Deletion: false},
			ID:        103,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
			Obsolete:  true,
		}
		Expect(indexLogPersistCreate(&indexlog3, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		indexlog4 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 10004, SourceDesc: "work10004", Deletion: false},
			ID:        104,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		Expect(indexLogPersistCreate(&indexlog4, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		indexlog5 := IndexLogRecord{
			IndexLog:  IndexLog{SourceType: "WORK", SourceId: 10005, SourceDesc: "work10005", Deletion: false},
			ID:        105,
			Timestamp: types.TimestampOfDate(2021, 1, 1, 12, 12, 12, 0, time.Local),
		}
		Expect(indexLogPersistCreate(&indexlog5, testDatabase.DS.GormDB(context.Background()))).To(BeNil())

		ret, err := LoadPendingIndexLog(1, 2)
		Expect(err).To(BeNil())
		Expect(len(ret)).To(Equal(2))
		Expect(ret[0]).To(Equal(indexlog1))
		Expect(ret[1]).To(Equal(indexlog4))

		ret, err = LoadPendingIndexLog(2, 2)
		Expect(err).To(BeNil())
		Expect(len(ret)).To(Equal(1))
		Expect(ret[0]).To(Equal(indexlog5))

		ret, err = LoadPendingIndexLog(0, 1)
		Expect(err).To(BeNil())
		Expect(len(ret)).To(Equal(1))
		Expect(ret[0]).To(Equal(indexlog1))
	})
}
