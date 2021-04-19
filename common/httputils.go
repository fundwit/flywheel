package common

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func HttpInvokeJson(method, url string, headers http.Header, reqBody string) (string, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(reqBody))
	if err != nil {
		return "", NewErrHttpInvoke(req, reqBody, nil, "", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	for name, values := range headers {
		req.Header.Del(name)
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", NewErrHttpInvoke(req, reqBody, resp, "", err)
	}

	defer resp.Body.Close()
	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", NewErrHttpInvoke(req, reqBody, resp, "", err)
	}
	respBody := string(respBodyBytes)
	if !HttpStatusIsSuccess(resp.StatusCode) {
		return "", NewErrHttpInvoke(req, reqBody, resp, respBody, nil)
	}

	return respBody, nil
}

func HttpStatusIsSuccess(status int) bool {
	return status >= 200 && status < 300
}

type ErrHttpInvoke struct {
	Method     string
	Url        string
	ReqHeaders http.Header
	ReqBody    string

	StatusCode  int
	StatusText  string
	RespHeaders http.Header
	RespBody    string

	Cause error
}

func NewErrHttpInvoke(req *http.Request, reqBody string, resp *http.Response, respBody string, cause error) *ErrHttpInvoke {
	err := ErrHttpInvoke{}
	err.Cause = cause
	if req != nil {
		err.Method = req.Method
		err.Url = req.URL.String()
		err.ReqHeaders = req.Header
		err.ReqBody = reqBody
	}

	if resp != nil {
		err.StatusCode = resp.StatusCode
		err.StatusText = resp.Status
		err.RespHeaders = resp.Header
		err.RespBody = respBody
	}
	return &err
}

func (e *ErrHttpInvoke) Error() string {
	method := ""
	url := ""
	reqHeader := http.Header{}

	statusCode := 0
	statusText := ""
	respHeader := http.Header{}

	return fmt.Sprintf("http invoke failed. request %s %s, headers: %s body: '%s'. response %d %s, headers: %s, body: '%s'",
		method, url, reqHeader, e.ReqBody, statusCode, statusText, respHeader, e.RespBody)
}
func (e *ErrHttpInvoke) Unwrap() error {
	return e.Cause
}
