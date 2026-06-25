package sls

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	lz4 "github.com/pierrec/lz4/v4"
	"github.com/stretchr/testify/require"
)

// newMockLogStore returns a LogStore + LogProject pair suitable for the
// in-process retry-loop mock tests below. The endpoint is bogus on
// purpose: the request() helper short-circuits whenever a mockErrorRetry
// is supplied, so no network call is ever made.
func newMockLogStore(t *testing.T, name string) *LogStore {
	t.Helper()
	project, err := NewLogProject("mock-project", "mock-endpoint.example.com", "mock-id", "mock-key")
	require.NoError(t, err)
	// Short retry/request budget so retry-exhaustion cases finish within the
	// test's `-timeout`. The mockErrorRetry hook short-circuits the actual
	// network call, so the only thing this constrains is the retry loop.
	project.WithRequestTimeout(500 * time.Millisecond).WithRetryTimeout(2 * time.Second)
	store, err := NewLogStore(name, project)
	require.NoError(t, err)
	return store
}

// TestLogStoreReadErrorMock exercises the GET-side retry loop in
// request() via the mockErrorRetry hook. Originally a method on the
// e2e-only LogstoreTestSuite — extracted here so it runs under the
// default build tag.
func TestLogStoreReadErrorMock(t *testing.T) {
	logstore := newMockLogStore(t, "mock-store")

	topic := ""
	begin_time := uint32(time.Now().Unix())
	from := int64(begin_time)
	to := int64(begin_time + 2)
	queryExp := "InternalServerError"
	maxLineNum := 100
	offset := 0
	reverse := false

	h := map[string]string{
		"x-log-bodyrawsize": "0",
		"Accept":            "application/json",
	}

	uri := fmt.Sprintf("/logstores/%v?type=log&topic=%v&from=%v&to=%v&query=%v&line=%v&offset=%v&reverse=%v",
		logstore.Name, topic, from, to, queryExp, maxLineNum, offset, reverse)

	mockErr := new(mockErrorRetry)
	mockErr.RetryCnt = 10000000
	serverError := Error{}
	serverError.Message = "server error 500"
	serverError.HTTPCode = int32(500)
	mockErr.Err = serverError

	// Retry exhaustion: error message reflects the timeout AND the underlying err.
	r1, err := request(logstore.project, "GET", uri, h, nil, mockErr)
	require.Nil(t, r1)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "context deadline exceeded"))
	require.True(t, strings.Contains(err.Error(), "server error 500"))
	require.True(t, strings.Contains(err.Error(), "stopped retrying err"))

	// Non-retriable error (404): bubbles up unchanged.
	mockErr.Err.HTTPCode = int32(404)
	mockErr.Err.Message = "server error 404"
	r2, err2 := request(logstore.project, "GET", uri, h, nil, mockErr)
	require.Nil(t, r2)
	require.Error(t, err2)
	require.False(t, strings.Contains(err2.Error(), "stopped retrying err"))
	require.False(t, strings.Contains(err2.Error(), "context deadline exceeded"))
	require.True(t, strings.Contains(err2.Error(), "server error 404"))

	// Success on first attempt: no retry happens.
	mockErr.Err.HTTPCode = int32(200)
	mockErr.RetryCnt = 1
	r3, err3 := request(logstore.project, "GET", uri, h, nil, mockErr)
	require.NotNil(t, r3)
	require.NoError(t, err3)

	// Transient retriable error then success.
	mockErr.Err.Message = "server error 500"
	mockErr.Err.HTTPCode = int32(500)
	mockErr.RetryCnt = 3

	r4, err4 := request(logstore.project, "GET", uri, h, nil, mockErr)
	require.NotNil(t, r4)
	require.NoError(t, err4)
}

// TestLogStoreWriteErrorMock exercises the POST-side retry loop in
// request() via mockErrorRetry. Lifted from the e2e LogstoreTestSuite.
func TestLogStoreWriteErrorMock(t *testing.T) {
	logstore := newMockLogStore(t, "mock-store")

	c := &LogContent{
		Key:   proto.String("error code"),
		Value: proto.String("InternalServerError"),
	}
	l := &Log{
		Time: proto.Uint32(uint32(time.Now().Unix())),
		Contents: []*LogContent{
			c,
		},
	}
	lg := &LogGroup{
		Topic:  proto.String("demo topic"),
		Source: proto.String("10.230.201.117"),
		Logs: []*Log{
			l,
		},
	}

	body, _ := proto.Marshal(lg)

	// Compresse body with lz4
	out := make([]byte, lz4.CompressBlockBound(len(body)))
	n, _ := lz4.CompressBlock(body, out, nil)

	h := map[string]string{
		"x-log-compresstype": "lz4",
		"x-log-bodyrawsize":  fmt.Sprintf("%v", len(body)),
		"Content-Type":       "application/x-protobuf",
	}

	uri := fmt.Sprintf("/logstores/%v", logstore.Name)

	mockErr := new(mockErrorRetry)
	mockErr.RetryCnt = 10000000
	serverError := Error{}
	serverError.Message = "server error 502"
	serverError.HTTPCode = int32(502)
	mockErr.Err = serverError

	// Retry exhaustion: combined error wrapper.
	r, err := request(logstore.project, "POST", uri, h, out[:n], mockErr)
	require.Nil(t, r)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "context deadline exceeded"))
	require.True(t, strings.Contains(err.Error(), "server error 502"))
	require.True(t, strings.Contains(err.Error(), "stopped retrying err"))

	// Non-retriable error (504): surfaced as-is.
	mockErr.Err.HTTPCode = int32(504)
	mockErr.Err.Message = "server error 504"
	r2, err2 := request(logstore.project, "POST", uri, h, out[:n], mockErr)

	require.Nil(t, r2)
	require.Error(t, err2)
	require.True(t, strings.Contains(err2.Error(), "server error 504"))
	require.False(t, strings.Contains(err2.Error(), "stopped retrying err"))
	require.False(t, strings.Contains(err2.Error(), "context deadline exceeded"))

	// Success on first attempt.
	mockErr.Err.HTTPCode = int32(200)
	mockErr.RetryCnt = 1
	r3, err3 := request(logstore.project, "POST", uri, h, out[:n], mockErr)

	require.NotNil(t, r3)
	require.NoError(t, err3)

	// Transient retriable error followed by success.
	mockErr.Err.Message = "server error 503"
	mockErr.Err.HTTPCode = int32(503)
	mockErr.RetryCnt = 3

	r4, err4 := request(logstore.project, "POST", uri, h, out[:n], mockErr)
	require.NotNil(t, r4)
	require.NoError(t, err4)
}
