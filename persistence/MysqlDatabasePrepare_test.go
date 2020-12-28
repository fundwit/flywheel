package persistence_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"flywheel/persistence"
)

var _ = Describe("MysqlDatabasePrepare", func() {
	Describe("ExtractDatabaseName", func() {
		It("should work correctly", func() {
			var name, rootUrl string
			var err error
			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)/dbname?charset=utf8mb4")
			Expect(err).To(BeNil())
			Expect(name).To(Equal("dbname"))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)/?charset=utf8mb4"))

			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)/?charset=utf8mb4")
			Expect(err).To(BeNil())
			Expect(name).To(Equal(""))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)/?charset=utf8mb4"))

			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)?charset=utf8mb4")
			Expect(err).To(BeNil())
			Expect(name).To(Equal(""))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)?charset=utf8mb4"))

			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)/dbname")
			Expect(err).To(BeNil())
			Expect(name).To(Equal("dbname"))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)/"))

			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)/")
			Expect(err).To(BeNil())
			Expect(name).To(Equal(""))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)/"))

			name, rootUrl, err = persistence.ExtractDatabaseName("root:P@4word@(test.xxxxx.com:3308)")
			Expect(err).To(BeNil())
			Expect(name).To(Equal(""))
			Expect(rootUrl).To(Equal("root:P@4word@(test.xxxxx.com:3308)"))

			// ...?.../...
			name, rootUrl, err = persistence.ExtractDatabaseName("root?abc/def")
			Expect(err).ToNot(BeNil())
			Expect(name).To(BeZero())
			Expect(rootUrl).To(BeZero())
		})
	})
})
