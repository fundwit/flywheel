package persistence

import (
	"errors"
	"os"
	"strings"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/sirupsen/logrus"
)

func PrepareMysqlDatabase(mysqlDriverArgs string) error {
	// root:xxx@(test.xxx.com:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	databaseName, rootDriverArgs, err := ExtractDatabaseName(mysqlDriverArgs)
	if err != nil {
		return err
	}

	db, err := gorm.Open("mysql", rootDriverArgs)
	if err != nil {
		return err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			logrus.Warnf("[prepare] failed to close connection after prepare mysql database: %v\n", err)
		}
	}()

	err = db.DB().Ping()
	if err != nil {
		return err
	}
	initSql := "CREATE DATABASE IF NOT EXISTS `" + databaseName + "` DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_unicode_ci;"

	if os.Getenv("GIN_MODE") == "debug" {
		db.LogMode(true)
	}

	err = db.Exec(initSql).Error
	if err != nil {
		return err
	}
	return nil
}

func ExtractDatabaseName(mysqlDriverArgs string) (string, string, error) {
	nameIndex := strings.IndexRune(mysqlDriverArgs, '/')
	paramsIndex := strings.IndexRune(mysqlDriverArgs, '?')

	// .../..?..
	if nameIndex > 0 && paramsIndex > nameIndex {
		return mysqlDriverArgs[nameIndex+1 : paramsIndex], mysqlDriverArgs[0:nameIndex+1] + mysqlDriverArgs[paramsIndex:], nil
	}
	// without /
	if nameIndex < 0 {
		return "", mysqlDriverArgs, nil
	}
	// with / and without ?
	if nameIndex > 0 && paramsIndex < 0 {
		return mysqlDriverArgs[nameIndex+1:], mysqlDriverArgs[0 : nameIndex+1], nil
	}

	// ..?../..
	return "", "", errors.New("invalid mysql driver args")
}
