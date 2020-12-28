package utils_test

import (
	"encoding/json"
	"flywheel/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testStruct struct {
	Id utils.ID
}

var _ = Describe("ID", func() {
	Describe("MarshalJSON", func() {
		It("should be MarshalJSON to string", func() {
			bytes, err := json.Marshal(&testStruct{Id: utils.ID(123)})
			Expect(err).To(BeNil())
			Expect(string(bytes)).To(MatchJSON(`{"Id":"123"}`))
		})
		It("should not be able to MarshalJSON to string when assigned to an interface{} variable", func() {
			bytes, err := json.Marshal(testStruct{Id: utils.ID(123)})
			Expect(err).To(BeNil())
			Expect(string(bytes)).To(MatchJSON(`{"Id":123}`))
		})
	})

	Describe("UnmarshalJSON", func() {
		It("should be UnmarshalJSON from string", func() {
			ts := testStruct{}
			err := json.Unmarshal([]byte(`{"Id":"123"}`), &ts)
			Expect(err).To(BeNil())
			Expect(ts.Id).To(Equal(utils.ID(123)))
		})
		It("should be UnmarshalJSON from number", func() {
			ts := testStruct{}
			err := json.Unmarshal([]byte(`{"Id":123}`), &ts)
			Expect(err).To(BeNil())
			Expect(ts.Id).To(Equal(utils.ID(123)))
		})
		It("should be UnmarshalJSON from non number value", func() {
			ts := testStruct{}
			err := json.Unmarshal([]byte(`{"Id":"abc"}`), &ts)
			Expect(err.Error()).To(Equal(`strconv.ParseUint: parsing "abc": invalid syntax`))
			Expect(ts.Id).To(BeZero())
		})
	})
})
