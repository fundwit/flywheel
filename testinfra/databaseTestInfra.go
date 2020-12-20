package testinfra

import (
	"flywheel/persistence"
	"github.com/google/uuid"
	"log"
	"os"
	"strings"
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
		log.Fatalf("failed to prepare database %v\n", err)
	}

	ds := &persistence.DataSourceManager{DatabaseConfig: dbConfig}
	// connect
	if err := ds.Start(); err != nil {
		defer ds.Stop()
		log.Fatalf("database conneciton failed %v\n", err)
	}

	return &TestDatabase{TestDatabaseName: databaseName, DS: ds}
}

func StopMysqlTestDatabase(testDatabase *TestDatabase) {
	if testDatabase != nil || testDatabase.DS != nil {
		if testDatabase.DS.GormDB() != nil {
			if err := testDatabase.DS.GormDB().Exec("DROP DATABASE " + testDatabase.TestDatabaseName).Error; err != nil {
				log.Println("failed to drop test database: " + testDatabase.TestDatabaseName)
			} else {
				log.Println("test database " + testDatabase.TestDatabaseName + " dropped")
			}
		}

		// close connection
		testDatabase.DS.Stop()
	}
}
