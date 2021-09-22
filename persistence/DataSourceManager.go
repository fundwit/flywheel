package persistence

import (
	"context"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	tracingGorm "github.com/smacker/opentracing-gorm"
)

var ActiveDataSourceManager *DataSourceManager

type DataSourceManager struct {
	gormDB *gorm.DB

	DatabaseConfig *DatabaseConfig
}

func (m *DataSourceManager) Start() error {
	db, err := connect(m.DatabaseConfig)
	if err != nil {
		return err
	}
	tracingGorm.AddGormCallbacks(db)
	m.gormDB = db
	if os.Getenv("GIN_MODE") == "debug" {
		m.gormDB.LogMode(true)
	}
	return nil
}

func (m *DataSourceManager) Stop() {
	if m.gormDB != nil {
		if err := m.gormDB.Close(); err != nil {
			logrus.Warnln("failed to close DB:", err)
		}
		m.gormDB = nil
	}
}

func (m *DataSourceManager) GormDB(c context.Context) *gorm.DB {
	if m.gormDB != nil {
		return tracingGorm.SetSpanToGorm(c, m.gormDB.New())
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
