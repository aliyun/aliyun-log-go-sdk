package sls

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func invalidJsonRespError(body string, header http.Header, httpCode int) error {
	return newBadResponseError(
		string(body),
		header,
		httpCode,
		fmt.Errorf("server returned an response with invalid JSON format"),
	)
}

func readResponseError(err error) error {
	return fmt.Errorf("fail to read response body: %w", err)
}

func httpStatusNotOkError(body []byte, header http.Header, httpCode int) error {
	slsErr := new(Error)
	if err := json.Unmarshal(body, slsErr); err != nil {
		return newBadResponseError(
			string(body),
			header,
			httpCode,
			fmt.Errorf("server returned an error response with invalid JSON format:%w", err),
		)
	}
	slsErr.HTTPCode = int32(httpCode)
	slsErr.RequestID = header.Get(RequestIDHeader)
	return slsErr
}

// BadResponseError : special sls error, not valid json format
type BadResponseError struct {
	RespBody     string
	RespHeader   map[string][]string
	HTTPCode     int
	ErrorMessage string
}

func (e BadResponseError) String() string {
	b, err := json.MarshalIndent(e, "", "    ")
	if err != nil {
		return ""
	}
	return string(b)
}

func (e BadResponseError) Error() string {
	return e.String()
}

// NewBadResponseError ...
func NewBadResponseError(body string, header map[string][]string, httpCode int) *BadResponseError {
	return &BadResponseError{
		RespBody:   body,
		RespHeader: header,
		HTTPCode:   httpCode,
	}
}

func newBadResponseError(body string, header map[string][]string, httpCode int, err error) *BadResponseError {
	return &BadResponseError{
		RespBody:     body,
		RespHeader:   header,
		HTTPCode:     httpCode,
		ErrorMessage: err.Error(),
	}
}

// mockErrorRetry : for mock the error retry logic
type mockErrorRetry struct {
	Err      Error
	RetryCnt int // RetryCnt-- after each retry. When RetryCnt > 0, return Err, else return nil, if set it BigUint, it equivalents to always failing.
}

func (e mockErrorRetry) Error() string {
	return e.Err.String()
}
