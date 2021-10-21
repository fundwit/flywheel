package domain

import (
	"errors"
	"strconv"
	"strings"

	"github.com/fundwit/go-commons/types"
)

const (
	PropTypeText     = "text"
	PropTypeTextArea = "textarea"
	PropTypeNumber   = "number"
	PropTypeTime     = "time"
)

type PropertyDefinition struct {
	Name string `json:"name" binding:"required" gorm:"unique_index:uni_workflow_prop"`
	Type string `json:"type" binding:"required,oneof=text textarea number time"`

	Title string `json:"title"`
}

var ErrUnsupportedPropertyType = errors.New("unsupported property type")

func (d PropertyDefinition) ValidateValue(raw string) (interface{}, error) {
	switch d.Type {
	case PropTypeText:
		return d.ValidateTextValue(raw)
	case PropTypeTextArea:
		return d.ValidateTextAreaValue(raw)
	case PropTypeNumber:
		return d.ValidateNumberValue(raw)
	case PropTypeTime:
		return d.ValidateTimeValue(raw)
	}

	return nil, ErrUnsupportedPropertyType
}

func (d PropertyDefinition) ValidateTextValue(raw string) (string, error) {
	return raw, nil
}

func (d PropertyDefinition) ValidateTextAreaValue(raw string) (string, error) {
	return d.ValidateTextValue(raw)
}

func (d PropertyDefinition) ValidateNumberValue(raw string) (interface{}, error) {
	if strings.Contains(raw, ".") {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	} else {
		v, err := strconv.ParseInt(raw, 0, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
}

func (d PropertyDefinition) ValidateTimeValue(raw string) (types.Timestamp, error) {
	var t types.Timestamp
	err := t.Scan(raw)
	return t, err
}
