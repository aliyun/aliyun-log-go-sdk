package sls

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/gogo/protobuf/proto"
	"github.com/jarcoal/httpmock"
	"github.com/pierrec/lz4/v4"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGetLogsTestSuite(t *testing.T) {
	suite.Run(t, new(GetLogsTestSuite))
}

// GetLogsTestSuite collects the pure unit tests around the v3 ->
// v2 response conversion plumbing. The e2e variant lives in
// log_store_e2e_test.go behind the `e2e` build tag.
type GetLogsTestSuite struct {
	suite.Suite
}

func (s *GetLogsTestSuite) TestConstructQueryInfo() {
	v3Meta := &GetLogsV3ResponseMeta{
		Keys:            nil,
		Terms:           nil,
		Marker:          nil,
		Mode:            nil,
		PhraseQueryInfo: nil,
		Shard:           nil,
		ScanBytes:       nil,
		IsAccurate:      nil,
		ColumnTypes:     nil,
		Highlights:      nil,
	}
	contents, err := v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{}", contents)
	b := false
	v3Meta.IsAccurate = &b
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{\"isAccurate\":0}", contents)
	b = true
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{\"isAccurate\":1}", contents)

	v3Meta.Keys = make([]string, 0)
	shard := 0
	v3Meta.Shard = &shard
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)

	s.Equal("{\"shard\":0,\"isAccurate\":1}", contents)
}

func (s *GetLogsTestSuite) TestMarshalLines() {
	logs := []map[string]string{
		{
			"key1": "va1",
			"key2": " sdsadsa",
		},
		{
			"keOIIO y1": "NKJ*((*va1",
			"ke y2":     " sdsadsa",
		},
		{
			"keA DSy1": "va 2>>122e",
			"key2":     " sds2adsa",
		},
	}
	data, err := json.Marshal(logs)
	s.Require().NoError(err)
	var msg []json.RawMessage
	err = json.Unmarshal(data, &msg)
	s.Require().NoError(err)
}

func TestLogStoreWriteCompression(t *testing.T) {
	client := CreateNormalInterface("http://example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	group := &LogGroup{Logs: []*Log{{Time: proto.Uint32(1)}}}
	raw, err := proto.Marshal(group)
	require.NoError(t, err)
	hashKey := "test-key"

	for _, tc := range []struct {
		method int
		header string
	}{
		{Compress_None, ""}, {Compress_LZ4, "lz4"}, {Compress_ZSTD, "zstd"}, {CompressionDeflate, "deflate"},
	} {
		transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, tc.header, req.Header.Get("x-log-compresstype"))
			require.Equal(t, strconv.Itoa(len(raw)), req.Header.Get(HTTPHeaderBodyRawSize))
			require.Equal(t, "application/x-protobuf", req.Header.Get(HTTPHeaderContentType))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			switch tc.method {
			case Compress_LZ4:
				decoded := make([]byte, len(raw))
				n, err := lz4.UncompressBlock(body, decoded)
				require.NoError(t, err)
				require.Equal(t, len(raw), n)
				body = decoded
			case Compress_ZSTD:
				body, err = slsZstdCompressor.Decompress(body, nil)
				require.NoError(t, err)
			case CompressionDeflate:
				body = decodeLogWriteDeflate(t, body)
			}
			require.Equal(t, raw, body)
			return httpmock.NewStringResponse(http.StatusOK, ""), nil
		})
		require.NoError(t, client.PutLogsWithCompressType("project", "store", group, tc.method))
		require.NoError(t, client.PostLogStoreLogsV2("project", "store", &PostLogStoreLogsRequest{
			LogGroup: group, HashKey: &hashKey, CompressType: tc.method,
		}))
		require.NoError(t, client.PutRawLogWithCompressType("project", "store", raw, tc.method))
		require.NoError(t, client.PostRawLogWithCompressType("project", "store", raw, tc.method, &hashKey))
	}
	require.ErrorIs(t, client.PutLogsWithCompressType("project", "store", group, Compress_Max), InvalidCompressError)
	require.Equal(t, 16, transport.GetTotalCallCount())
}
