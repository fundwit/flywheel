package domain

import (
	"errors"
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
		v, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30:40")
		rts, _ = v.(types.Timestamp)
		Expect(rts.Time().Equal(ts0.Time())).To(BeTrue())
		Expect(err).To(BeNil())

		_, err = PropertyDefinition{Type: PropTypeTime}.ValidateValue("2020-02-03 20:30")
		Expect(err.Error()).To(Equal(`parsing time "2020-02-03 20:30" as "2006-01-02 15:04:05.000000000": cannot parse "" as ":"`))
	})
}
