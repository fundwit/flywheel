package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"flywheel/bizerror"
	"fmt"
	"strconv"
	"strings"

	"github.com/fundwit/go-commons/types"
)

const (
	PropTypeText     = "text"
	PropTypeTextArea = "textarea"
	PropTypeNumber   = "number"
	PropTypeTime     = "time"
	PropTypeSelect   = "select"

	OptionKeySelectEnum = "select_enums"
)

type PropertyDefinition struct {
	Name string `json:"name" binding:"required" gorm:"unique_index:uni_workflow_prop"`
	Type string `json:"type" binding:"required,oneof=text textarea number time select"`

	Title   string          `json:"title"`
	Options PropertyOptions `json:"options" sql:"type:VARCHAR(1024)"`
}

type PropertyOptions map[string]interface{}

func (t PropertyDefinition) ValidateOptions() error {
	if t.Type == PropTypeSelect {
		enums, ok := t.Options[OptionKeySelectEnum].([]string)
		if !ok || len(enums) == 0 {
			return bizerror.ErrInvalidArguments
		}

		uniSet := map[string]bool{}
		for _, item := range enums {
			if len(item) == 0 || strings.TrimSpace(item) != item {
				return bizerror.ErrInvalidArguments
			}

			if _, ok := uniSet[strings.ToLower(item)]; ok {
				return bizerror.ErrInvalidArguments
			}

			uniSet[strings.ToLower(item)] = true
		}
	}
	return nil
}

func (t PropertyOptions) Value() (driver.Value, error) {
	jsonBytes, err := json.Marshal(&t)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

func (c *PropertyOptions) Scan(v interface{}) error {
	jsonString, ok := v.(string)
	if !ok {
		jsonByte, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("type is neither string nor []byte: %T %v", v, v)
		}
		jsonString = string(jsonByte)
	}
	return json.Unmarshal([]byte(jsonString), c)
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
	case PropTypeSelect:
		return d.ValidateSelectValue(raw)
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

func (d PropertyDefinition) ValidateSelectValue(raw string) (string, error) {
	enums, ok := d.Options[OptionKeySelectEnum].([]string)
	if !ok {
		return "", errors.New("invalid property definition")
	}

	for _, r := range enums {
		if r == raw {
			return raw, nil
		}
	}

	return "", &types.ErrInvalidParameter{Parameter: raw}
}
