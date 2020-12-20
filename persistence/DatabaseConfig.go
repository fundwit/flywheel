package persistence

import (
	"errors"
	"os"
	"strings"
)

type DatabaseConfig struct {
	DriverType string
	DriverArgs string
}

const EnvDatabaseURL = "DATABASE_URL"

func ParseDatabaseConfigFromEnv() (*DatabaseConfig, error) {
	databaseUrl := os.ExpandEnv(os.Getenv(EnvDatabaseURL))

	slice := strings.Split(databaseUrl, "://")
	if len(slice) != 2 {
		return nil, errors.New(EnvDatabaseURL + " is not valid, a correct example like 'mysql://user:pwd@tcp(host:3306)/dbname?para=value'")
	}

	return &DatabaseConfig{DriverType: strings.ToLower(slice[0]), DriverArgs: slice[1]}, nil
}
