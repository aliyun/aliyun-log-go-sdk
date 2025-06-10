package sls

import (
	"fmt"

	"github.com/pierrec/lz4/v4"
)

func doCompress(body []byte, compressType int) ([]byte, error) {
	switch compressType {
	case Compress_LZ4:
		// Compresse body with lz4
		out := make([]byte, lz4.CompressBlockBound(len(body)))
		var hashTable [1 << 16]int
		n, err := lz4.CompressBlock(body, out, hashTable[:])
		if err != nil {
			return nil, NewClientError(err)
		}
		// copy incompressible data as lz4 format
		if n == 0 {
			n, _ = copyIncompressible(body, out)
		}
		return out[:n], nil

	case Compress_ZSTD:
		// Compress body with zstd
		out, err := slsZstdCompressor.Compress(body, nil)
		if err != nil {
			return nil, NewClientError(err)
		}
		return out, nil

	case Compress_None:
		return body, nil
	}
	return nil, NewClientError(fmt.Errorf("Unsupported compress type: %d", compressType))
}

// internal use only
func CompressLogGroup(body []byte, compressType int) (*CompressedLogGroup, error) {
	data, err := doCompress(body, compressType)
	if err != nil {
		return nil, err
	}
	return &CompressedLogGroup{
		CompressType:   compressType,
		RawSize:        len(body),
		CompressedData: data,
	}, nil
}
