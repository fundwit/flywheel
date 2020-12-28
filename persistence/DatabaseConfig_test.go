package persistence_test

import (
	"flywheel/persistence"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"strings"
)

var _ = Describe("DatabaseConfig", func() {
	BeforeEach(func() {
		// reset
		os.Setenv(persistence.EnvDatabaseURL, "")
	})

	It("should return a DatabaseConfig instance when database url env is valid", func() {
		os.Setenv(persistence.EnvDatabaseURL, "Mysql://user.pwd@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=true")
		config, err := persistence.ParseDatabaseConfigFromEnv()

		Expect(err).To(BeZero())
		Expect(config).ToNot(BeNil())
		Expect(config.DriverType).To(Equal("mysql"))
		Expect(config.DriverArgs).To(Equal("user.pwd@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=true"))
	})

	It("should return err when database url env is not valid", func() {
		os.Setenv(persistence.EnvDatabaseURL, "user.pwd@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=true")
		config, err := persistence.ParseDatabaseConfigFromEnv()

		Expect(err).ToNot(BeZero())
		Expect(config).To(BeNil())
		Expect(strings.Contains(err.Error(), "is not valid, a correct example like")).To(BeTrue())
	})
})
