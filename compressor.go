package sls

import (
	"bytes"
	"compress/zlib"
	"strconv"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

var slsZstdCompressor LogCompressor = NewZstdCompressor(zstd.SpeedFastest)

func SetZstdCompressor(compressor LogCompressor) error {
	slsZstdCompressor = compressor
	return nil
}

type LogCompressor interface {
	// Compress src into dst.  If you have a buffer to use, you can pass it to
	// prevent allocation.  If it is too small, or if nil is passed, a new buffer
	// will be allocated and returned.
	Compress(src, dst []byte) ([]byte, error)
	// Decompress src into dst.  If you have a buffer to use, you can pass it to
	// prevent allocation.  If it is too small, or if nil is passed, a new buffer
	// will be allocated and returned.
	Decompress(src, dst []byte) ([]byte, error)
}

type ZstdCompressor struct {
	writer *zstd.Encoder
	reader *zstd.Decoder
	level  zstd.EncoderLevel
}

func NewZstdCompressor(level zstd.EncoderLevel) *ZstdCompressor {
	res := &ZstdCompressor{
		level: level,
	}
	res.writer, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(res.level))
	res.reader, _ = zstd.NewReader(nil)
	return res
}

func (c *ZstdCompressor) Compress(src, dst []byte) ([]byte, error) {
	if dst != nil {
		return c.writer.EncodeAll(src, dst[:0]), nil
	}
	return c.writer.EncodeAll(src, nil), nil
}

func (c *ZstdCompressor) Decompress(src, dst []byte) ([]byte, error) {
	if dst != nil {
		return c.reader.DecodeAll(src, dst[:0])
	}
	return c.reader.DecodeAll(src, nil)
}

// logWriteCompression relates a Go SDK option to SLS HTTP and package encodings.
type logWriteCompression struct {
	method       int
	header       string
	packageValue uint64
}

func resolveLogWriteCompression(method int) (logWriteCompression, error) {
	// Package values follow apsara.sls.pb.SlsCompressType.
	switch method {
	case Compress_None:
		return logWriteCompression{method: method, packageValue: 0}, nil
	case CompressionDeflate:
		return logWriteCompression{method: method, header: "deflate", packageValue: 1}, nil
	case Compress_LZ4:
		return logWriteCompression{method: method, header: "lz4", packageValue: 2}, nil
	case Compress_ZSTD:
		return logWriteCompression{method: method, header: "zstd", packageValue: 3}, nil
	default:
		return logWriteCompression{}, InvalidCompressError
	}
}

func (c logWriteCompression) compress(raw []byte) ([]byte, error) {
	switch c.method {
	case Compress_None:
		return raw, nil
	case Compress_LZ4:
		out := make([]byte, lz4.CompressBlockBound(len(raw)))
		n, err := lz4.CompressBlock(raw, out, nil)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			n, _ = copyIncompressible(raw, out)
		}
		return out[:n], nil
	case Compress_ZSTD:
		return slsZstdCompressor.Compress(raw, nil)
	case CompressionDeflate:
		var out bytes.Buffer
		writer := zlib.NewWriter(&out)
		if _, err := writer.Write(raw); err != nil {
			_ = writer.Close()
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	default:
		return nil, InvalidCompressError
	}
}

func logWriteHeaders(rawSize int, encoding string) map[string]string {
	headers := map[string]string{
		HTTPHeaderContentType: "application/x-protobuf",
		HTTPHeaderBodyRawSize: strconv.Itoa(rawSize),
	}
	if encoding != "" {
		headers["x-log-compresstype"] = encoding
	}
	return headers
}

func encodeLogWrite(raw []byte, method int) ([]byte, map[string]string, error) {
	compression, err := resolveLogWriteCompression(method)
	if err != nil {
		return nil, nil, err
	}
	body, err := compression.compress(raw)
	if err != nil {
		return nil, nil, err
	}
	return body, logWriteHeaders(len(raw), compression.header), nil
}

func copyIncompressible(src, dst []byte) (int, error) {
	lLen, dn := len(src), len(dst)

	di := 0
	if lLen < 0xF {
		dst[di] = byte(lLen << 4)
	} else {
		dst[di] = 0xF0
		if di++; di == dn {
			return di, nil
		}
		lLen -= 0xF
		for ; lLen >= 0xFF; lLen -= 0xFF {
			dst[di] = 0xFF
			if di++; di == dn {
				return di, nil
			}
		}
		dst[di] = byte(lLen)
	}
	if di++; di+len(src) > dn {
		return di, nil
	}
	di += copy(dst[di:], src)
	return di, nil
}
