package indexlog

import (
	"flywheel/persistence"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

type IndexLog struct {
	SourceType string   `json:"sourceType" gorm:"index:for_search"`
	SourceId   types.ID `json:"sourceId" gorm:"index:for_search"`
	SourceDesc string   `json:"sourceDesc"`

	Deletion bool `json:"deletion"`
}

type IndexLogRecord struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	IndexLog

	Obsolete    bool            `json:"obsolete"`
	Timestamp   types.Timestamp `json:"timestamp" sql:"type:DATETIME(6)"`
	IndexedTime types.Timestamp `json:"indexedTime" sql:"type:DATETIME(6)"`
}

func (r *IndexLogRecord) TableName() string {
	return "index_logs"
}

var (
	CreateIndexLogFunc        = CreateIndexLog
	FinishIndexLogFunc        = FinishIndexLog
	IndexLogPersistCreateFunc = indexLogPersistCreate
)

func CreateIndexLog(id types.ID, sourceType string, sourceId types.ID, sourceDesc string, deletion bool,
	timestamp types.Timestamp, tx *gorm.DB) (*IndexLogRecord, error) {

	record := IndexLogRecord{
		IndexLog: IndexLog{
			SourceType: sourceType,
			SourceId:   sourceId,
			SourceDesc: sourceDesc,
			Deletion:   deletion,
		},
		ID:        id,
		Timestamp: timestamp,
	}

	if err := IndexLogPersistCreateFunc(&record, tx); err != nil {
		return nil, err
	}
	return &record, nil
}

func FinishIndexLog(id types.ID) error {
	changes := map[string]interface{}{"indexed_time": types.CurrentTimestamp(), "obsolete": false}
	if err := persistence.ActiveDataSourceManager.GormDB().Model(&IndexLogRecord{}).Where("id = ?", id).Update(changes).Error; err != nil {
		return err
	}
	return nil
}

func indexLogPersistCreate(record *IndexLogRecord, tx *gorm.DB) error {
	// obsolete all old record
	if err := tx.Model(&IndexLogRecord{}).Where("source_type LIKE ? AND source_id = ? AND indexed_time <= '0001-01-01 00:00:00.000000'",
		record.SourceType, record.SourceId).Update("obsolete", true).Error; err != nil {
		return err
	}

	return tx.Create(record).Error
}