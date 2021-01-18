package common

import "net/http"

type BizError interface {
	Respond() *BizErrorDetail
}

type BizErrorDetail struct {
	Status  int
	Code    string
	Message string

	Data  interface{}
	Cause error
}

type ErrBadParam struct {
	Cause error
}

func (e *ErrBadParam) Unwrap() error {
	return e.Cause
}
func (e *ErrBadParam) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "common.bad_param"
}
func (e *ErrBadParam) Respond() *BizErrorDetail {
	message := "common.bad_param"
	if e.Cause != nil {
		message = e.Cause.Error()
	}
	return &BizErrorDetail{Status: http.StatusBadRequest, Code: "common.bad_param", Message: message, Data: nil}
}