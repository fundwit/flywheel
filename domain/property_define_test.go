package domain

import (
	"errors"
	"flywheel/bizerror"
	"strconv"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
)

func TestPropertyDefinition_ValidateValue(t *testing.T) {
	RegisterTestingT(t)

	t.Run("be able to validate values", func(t *testing.T) {
		Expect(PropertyDefinition{Type: PropTypeText}.ValidateValue("aaa")).To(Equal("aaa"))
		Expect(PropertyDefinition{Type: PropTypeTextArea}.ValidateValue("aaa")).To(Equal("aaa"))

		Expect(PropertyDefinition{Type: PropTypeNumber}.ValidateValue("123")).To(Equal(int64(123)))
		Expect(PropertyDefinition{Type: PropTypeNumber}.ValidateValue("0xAB")).To(Equal(int64(171)))
		Expect(PropertyDefinition{Type: PropTypeNumber}.ValidateValue("12.3")).To(Equal(float64(12.3)))

		_, err := PropertyDefinition{Type: PropTypeNumber}.ValidateValue("12.3ab")
		Expect(errors.Unwrap(err)).To(Equal(strconv.ErrSyntax))

		_, err = PropertyDefinition{Type: PropTypeNumber}.ValidateValue("abc")
		Expect(errors.Unwrap(err)).To(Equal(strconv.ErrSyntax))
		_, err = PropertyDefinition{Type: PropTypeNumber}.ValidateValue("")
		Expect(errors.Unwrap(err)).To(Equal(strconv.ErrSyntax))

		_, err = PropertyDefinition{Type: "notsupported"}.ValidateValue("abc")
		Expect(err).To(Equal(ErrUnsupportedPropertyType))
	})

	t.Run("be able to validate time value", func(t *testing.T) {
		ts := types.TimestampOfDate(2020, 2, 3, 20, 30, 40, 123000000, time.Local)
		ts0 := types.TimestampOfDate(2020, 2, 3, 20, 30, 40, 0, time.Local)

		v, err := PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30:40.123000000")
		rts, _ := v.(types.Timestamp)
		Expect(rts.Time().Equal(ts.Time())).To(BeTrue())
		Expect(err).To(BeNil())
		v, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30:40.1230")
		rts, _ = v.(types.Timestamp)
		Expect(rts.Time().Equal(ts.Time())).To(BeTrue())
		Expect(err).To(BeNil())

		localTZ := time.Now().Format(time.RFC3339)[19:]
		v, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03T20:30:40.123" + localTZ)
		rts, _ = v.(types.Timestamp)
		Expect(rts.Time().Equal(ts.Time())).To(BeTrue())
		Expect(err).To(BeNil())

		v, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30:40")
		rts, _ = v.(types.Timestamp)
		Expect(rts.Time().Equal(ts0.Time())).To(BeTrue())
		Expect(err).To(BeNil())

		_, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30")
		Expect(err.Error()).To(Equal(`invalid parameter: "2020-02-03 20:30"`))
	})

	t.Run("be able to validate select value", func(t *testing.T) {
		ops := map[string]interface{}{OptionKeySelectEnum: []string{"Cat", "Dog"}}
		v, err := PropertyDefinition{Type: PropTypeSelect, Options: ops}.ValidateValue("Cat")
		Expect(v).To(Equal("Cat"))
		Expect(err).To(BeNil())

		v, err = PropertyDefinition{Type: PropTypeSelect, Options: ops}.ValidateValue("Fish")
		Expect(v).To(BeZero())
		Expect(err.Error()).To(Equal(`invalid parameter: "Fish"`))

		ops = map[string]interface{}{OptionKeySelectEnum: []int{100, 200}}
		v, err = PropertyDefinition{Type: PropTypeSelect, Options: ops}.ValidateValue("Fish")
		Expect(v).To(BeZero())
		Expect(err.Error()).To(Equal(`invalid property definition`))
	})
}

func TestPropertyDefinition_ValidateOptions(t *testing.T) {
	RegisterTestingT(t)

	t.Run("be able to validate options", func(t *testing.T) {
		d := PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{"Cat", "Dog"}}}
		Expect(d.ValidateOptions()).To(BeNil())

		Expect(PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{}}}.ValidateOptions()).
			To(Equal(bizerror.ErrInvalidArguments))

		Expect(PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{"Cat", ""}}}.ValidateOptions()).
			To(Equal(bizerror.ErrInvalidArguments))

		Expect(PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{" Cat "}}}.ValidateOptions()).
			To(Equal(bizerror.ErrInvalidArguments))

		Expect(PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{"Cat", "cat"}}}.ValidateOptions()).
			To(Equal(bizerror.ErrInvalidArguments))

		Expect(PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []int{123}}}.ValidateOptions()).
			To(Equal(bizerror.ErrInvalidArguments))
	})
}

func TestPropertyOptions_Value(t *testing.T) {
	RegisterTestingT(t)

	t.Run("value func should work as expected", func(t *testing.T) {
		d := PropertyDefinition{Type: PropTypeSelect,
			Options: map[string]interface{}{OptionKeySelectEnum: []string{"Cat", "Dog"}}}
		Expect(d.Options.Value()).To(MatchJSON(`{"selectEnums": ["Cat", "Dog"]}`))

	})
}

func TestPropertyOptions_Scan(t *testing.T) {
	RegisterTestingT(t)

	t.Run("scan func should work as expected", func(t *testing.T) {
		json := `{"selectEnums": ["Cat", "Dog"]}`

		options := PropertyOptions{}
		Expect(options.Scan(json)).To(BeNil())
		Expect(options).To(Equal(PropertyOptions(map[string]interface{}{OptionKeySelectEnum: []interface{}{"Cat", "Dog"}})))

		options = PropertyOptions{}
		Expect(options.Scan([]byte(json))).To(BeNil())
		Expect(options).To(Equal(PropertyOptions(map[string]interface{}{OptionKeySelectEnum: []interface{}{"Cat", "Dog"}})))

		Expect(options.Scan(123)).To(Equal(errors.New("type is neither string nor []byte: int 123")))
	})
}
