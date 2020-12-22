package persistence

import (
	"github.com/jinzhu/gorm"
	"log"
	"os"
)

type DataSourceManager struct {
	gormDB *gorm.DB

	DatabaseConfig *DatabaseConfig
}

func (m *DataSourceManager) Start() error {
	db, err := connect(m.DatabaseConfig)
	if err != nil {
		return err
	}
	m.gormDB = db
	if os.Getenv("GIN_MODE") != "release" {
		m.gormDB.LogMode(true)
	}
	return nil
}

func (m *DataSourceManager) Stop() {
	if m.gormDB != nil {
		if err := m.gormDB.Close(); err != nil {
			log.Printf("fialed to close DB: %v", err)
		}
		m.gormDB = nil
	}
}

func (m *DataSourceManager) GormDB() *gorm.DB {
	if m.gormDB != nil {
		return m.gormDB.New()
	}
	return nil
}

func connect(config *DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(config.DriverType, config.DriverArgs)
	if err != nil {
		return nil, err
	}
	err = db.DB().Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}
