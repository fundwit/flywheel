package common_test

import (
	"flywheel/common"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timestamp", func() {
	Describe("Value", func() {
		It("should be able to calculate value correctly", func() {
			v, err := common.Timestamp{}.Value()
			Expect(err).To(BeNil())
			Expect(v).To(Equal("0001-01-01 00:00:00.000000000"))

			v, err = common.TimestampOfDate(2021, 5, 6, 12, 30, 40, 666666666, time.Local).Value()
			Expect(err).To(BeNil())
			Expect(v).To(Equal("2021-05-06 12:30:40.666667000"))
		})
	})

	Describe("Scan", func() {
		It("should be able to scan value", func() {
			t := common.TimestampOfDate(2021, 1, 1, 12, 30, 40, 666666666, time.Local)

			Expect(t.Scan("0001-01-01 00:00:00.000000000")).To(BeNil())
			Expect(t.Time().IsZero()).To(BeTrue())
			Expect(t).To(Equal(common.Timestamp{}))

			Expect(t.Scan("0001-01-01 01:02:03.004")).To(BeNil())
			Expect(t.Time().IsZero()).To(BeTrue())
			Expect(t).To(Equal(common.Timestamp{}))

			Expect(t.Scan("0000-01-01 00:00:00")).To(BeNil())
			Expect(t.Time().IsZero()).To(BeTrue())
			Expect(t).To(Equal(common.Timestamp{}))

			Expect(t.Scan("2021-05-06 12:30:40.666666666")).To(BeNil())
			Expect(t).To(Equal(common.TimestampOfDate(2021, 5, 6, 12, 30, 40, 666666666, time.Local)))
		})
	})

	Describe("CurrentTimestamp", func() {
		It("should be able to calculate value correctly", func() {
			begin := time.Now().Round(time.Microsecond)
			v := common.CurrentTimestamp()
			end := time.Now().Round(time.Microsecond)

			Expect(v.Time().UnixNano() >= begin.UnixNano()).To(BeTrue())
			Expect(v.Time().UnixNano() <= end.UnixNano()).To(BeTrue())
		})
	})

	Describe("MarshalJSON and UnmarshalJSON", func() {
		It("should be able to marshal json", func() {
			t := common.TimestampOfDate(2021, 1, 1, 12, 30, 40, 666666666, time.UTC)
			jsonBytes, err := t.MarshalJSON()
			Expect(err).To(BeNil())
			Expect(string(jsonBytes)).To(Equal(`"2021-01-01T12:30:40.666667Z"`))

			var t1 common.Timestamp
			Expect(t1.UnmarshalJSON(jsonBytes)).To(BeNil())
			Expect(t1).To(Equal(t))

			jsonBytes, err = common.Timestamp{}.MarshalJSON()
			Expect(err).To(BeNil())
			Expect(string(jsonBytes)).To(Equal(`null`))

			var t2 common.Timestamp
			Expect(t2.UnmarshalJSON(jsonBytes)).To(BeNil())
			Expect(t2.Time().IsZero()).To(BeTrue())
		})
	})
	Describe("MarshalText and UnmarshalText", func() {
		It("should be able to marshal test", func() {
			t := common.TimestampOfDate(2021, 1, 1, 12, 30, 40, 666666666, time.UTC)
			textBytes, err := t.MarshalText()
			Expect(err).To(BeNil())
			Expect(string(textBytes)).To(Equal(`2021-01-01T12:30:40.666667Z`))

			var t1 common.Timestamp
			Expect(t1.UnmarshalText(textBytes)).To(BeNil())
			Expect(t1).To(Equal(t))
		})
	})
})
