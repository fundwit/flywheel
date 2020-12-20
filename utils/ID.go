package utils

import (
	"strconv"
	"strings"
)

type ID uint64

func (id *ID) UnmarshalJSON(data []byte) (err error) {
	s := strings.Trim(string(data), `"`)
	ID, err := ParseID(s)
	if err != nil {
		return err
	}
	*id = ID
	return
}

func (id *ID) MarshalJSON() ([]byte, error) {
	return ([]byte)(`"` + strconv.FormatUint(uint64(*id), 10) + `"`), nil
}

func ParseID(idString string) (ID, error) {
	idUint64, err := strconv.ParseUint(idString, 10, 64)
	if err != nil {
		return 0, err
	}
	return ID(idUint64), nil
}
