package persistence_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("DataSourceManager", func() {
	Describe("Start and connect to Database", func() {
		Context("mysql database", func() {
			It("should be able to connect to database", func() {
				// database must exist
				//os.Setenv(persistence.EnvDatabaseURL, "mysql://....")
				//config, err := persistence.ParseDatabaseConfigFromEnv()
				//Expect(err).To(BeNil())
				//
				//ds := &persistence.DataSourceManager{
				//	DatabaseConfig: config,
				//}
				//Expect(ds.GormDB()).To(BeNil())
				//
				//if err := ds.Start(); err != nil {
				//	logrus.Fatal(err)
				//}
				//defer ds.Stop()
				//Expect(ds.GormDB()).ToNot(BeNil())
				//
				//ds.Stop()
				//Expect(ds.GormDB()).To(BeNil())
			})
		})
	})
})
