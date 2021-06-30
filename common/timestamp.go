package common

import (
	"database/sql/driver"
	"errors"
	"time"
)

type Timestamp time.Time

const format = "2006-01-02 15:04:05.000000000"

func CurrentTimestamp() Timestamp {
	return Timestamp(time.Now().Round(time.Microsecond))
}
func TimestampOfDate(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) Timestamp {
	return (Timestamp)(time.Date(year, month, day, hour, min, sec, nsec, loc).Round(time.Microsecond))
}
func (t Timestamp) Time() time.Time {
	return (time.Time)(t)
}

func (t Timestamp) String() string {
	return t.Time().String()
}
func (t Timestamp) IsZero() bool {
	return t.Time().IsZero()
}

// func (t Timestamp) GormDataType(dialect gorm.Dialect) string {
// 	return "DATETIME(6)"
// }

func (t Timestamp) Value() (driver.Value, error) {
	d := t.Time()

	// zero time normalize
	if d.Year() <= 1 && d.Month() <= 1 && d.Day() <= 1 {
		d = time.Time{} // 0,0,UTC
	}

	return d.Format(format), nil
}

func (c *Timestamp) Scan(v interface{}) error {
	parsedTime, ok := v.(time.Time)
	if !ok {
		timeString, ok := v.(string)
		if !ok {
			return errors.New("unsupported type")
		}
		len := len(timeString)
		if len == 19 {
			timeString = timeString + ".000000000"
		} else if len == 21 {
			timeString = timeString + "00000000"
		} else if len == 22 {
			timeString = timeString + "0000000"
		} else if len == 23 {
			timeString = timeString + "000000"
		} else if len == 24 {
			timeString = timeString + "00000"
		} else if len == 25 {
			timeString = timeString + "0000"
		} else if len == 26 {
			timeString = timeString + "000"
		} else if len == 27 {
			timeString = timeString + "00"
		} else if len == 28 {
			timeString = timeString + "0"
		}
		var err error
		parsedTime, err = time.ParseInLocation(format, timeString, time.Local)
		if err != nil {
			return err
		}
		parsedTime = parsedTime.Round(time.Microsecond)
	}

	// zero time normalize
	if parsedTime.Year() <= 1 && parsedTime.Month() <= 1 && parsedTime.Day() <= 1 {
		*c = Timestamp(time.Time{}) // 0,0,UTC
	} else {
		*c = Timestamp(parsedTime)
	}
	return nil
}

func (t Timestamp) MarshalBinary() ([]byte, error) {
	return t.Time().MarshalBinary()
}

func (t *Timestamp) UnmarshalBinary(data []byte) error {
	var d time.Time
	if err := d.UnmarshalBinary(data); err != nil {
		return err
	}
	*t = Timestamp(d)
	return nil
}

func (t Timestamp) GobEncode() ([]byte, error) {
	return t.Time().GobEncode()
}

func (t *Timestamp) GobDecode(data []byte) error {
	var d time.Time
	if err := d.GobDecode(data); err != nil {
		return err
	}
	*t = Timestamp(d)
	return nil
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.Time().IsZero() {
		return []byte("null"), nil
	}
	return t.Time().MarshalJSON()
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = Timestamp{}
		return nil
	}

	var d time.Time
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	*t = Timestamp(d)
	return nil
}

func (t Timestamp) MarshalText() ([]byte, error) {
	return t.Time().MarshalText()
}

func (t *Timestamp) UnmarshalText(data []byte) error {
	var d time.Time
	if err := d.UnmarshalText(data); err != nil {
		return err
	}
	*t = Timestamp(d)
	return nil
}
