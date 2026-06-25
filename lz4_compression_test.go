package sls

import (
	"bytes"
	"math/rand"
	"net/http"
	"strconv"
	"testing"

	"github.com/pierrec/lz4/v4"
	"github.com/stretchr/testify/require"
)

func TestLZ4CompressBlockNilRoundTrip(t *testing.T) {
	cases := map[string][]byte{
		"compressible": bytes.Repeat([]byte("aliyun-log-service-"), 256),
		"mixed":        deterministicBytes(32 * 1024),
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			dst := make([]byte, lz4.CompressBlockBound(len(src)))
			n, err := lz4.CompressBlock(src, dst, nil)
			require.NoError(t, err)
			require.NotZero(t, n)

			got, err := decompressLZ4ForTest(len(src), dst[:n])
			require.NoError(t, err)
			require.Equal(t, src, got)
		})
	}
}

func TestLZ4CopyIncompressibleRoundTrip(t *testing.T) {
	src := deterministicBytes(64 * 1024)
	dst := make([]byte, lz4.CompressBlockBound(len(src)))

	n, err := copyIncompressible(src, dst)
	require.NoError(t, err)
	require.NotZero(t, n)

	got, err := decompressLZ4ForTest(len(src), dst[:n])
	require.NoError(t, err)
	require.Equal(t, src, got)
}

func TestLZ4DecompressResponseRejectsSizeMismatch(t *testing.T) {
	src := bytes.Repeat([]byte("log-data-"), 128)
	dst := make([]byte, lz4.CompressBlockBound(len(src)))
	n, err := lz4.CompressBlock(src, dst, nil)
	require.NoError(t, err)
	require.NotZero(t, n)

	_, err = decompressLZ4ForTest(len(src)+1, dst[:n])
	require.ErrorContains(t, err, "does not match")
}

func decompressLZ4ForTest(rawSize int, body []byte) ([]byte, error) {
	return decompressResponse(rawSize, body, &http.Response{
		Header: http.Header{
			"X-Log-Compresstype": []string{"lz4"},
			"X-Log-Bodyrawsize":  []string{strconv.Itoa(rawSize)},
		},
	})
}

func deterministicBytes(n int) []byte {
	out := make([]byte, n)
	r := rand.New(rand.NewSource(1))
	_, _ = r.Read(out)
	return out
}
