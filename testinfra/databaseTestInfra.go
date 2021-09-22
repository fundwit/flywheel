package testinfra

import (
	"context"
	"flywheel/persistence"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type TestDatabase struct {
	TestDatabaseName string
	DS               *persistence.DataSourceManager
}

// StartMysqlTestDatabase TEST_MYSQL_SERVICE=root:root@(127.0.0.1:3306)
func StartMysqlTestDatabase(baseName string) *TestDatabase {
	mysqlSvc := os.Getenv("TEST_MYSQL_SERVICE")
	if mysqlSvc == "" {
		mysqlSvc = "root:root@(127.0.0.1:3306)"
	}
	databaseName := baseName + "_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	dbConfig := &persistence.DatabaseConfig{
		DriverType: "mysql", DriverArgs: mysqlSvc + "/" + databaseName + "?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s",
	}

	// create database (no conflict)
	if err := persistence.PrepareMysqlDatabase(dbConfig.DriverArgs); err != nil {
		logrus.Fatalf("failed to prepare database %v\n", err)
	}

	ds := &persistence.DataSourceManager{DatabaseConfig: dbConfig}
	// connect
	if err := ds.Start(); err != nil {
		defer ds.Stop()
		logrus.Fatalf("database connection failed %v\n", err)
	}

	return &TestDatabase{TestDatabaseName: databaseName, DS: ds}
}

func StopMysqlTestDatabase(testDatabase *TestDatabase) {
	if testDatabase != nil || testDatabase.DS != nil {
		if testDatabase.DS.GormDB(context.Background()) != nil {
			if err := testDatabase.DS.GormDB(context.Background()).Exec("DROP DATABASE " + testDatabase.TestDatabaseName).Error; err != nil {
				logrus.Println("failed to drop test database: " + testDatabase.TestDatabaseName)
			} else {
				logrus.Debugln("test database " + testDatabase.TestDatabaseName + " dropped")
			}
		}

		// close connection
		testDatabase.DS.Stop()
	}
}
