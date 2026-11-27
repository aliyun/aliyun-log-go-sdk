package sls

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/pierrec/lz4/v4"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	// GraphStoreMaxWriteBytes bounds both uncompressed input and the HTTP request body.
	GraphStoreMaxWriteBytes = 10 * 1024 * 1024
	graphStoreMaxWriteLogs  = 100000

	// Compress_GZIP is GraphStore's zlib-wrapped DEFLATE format, matching the Java SDK.
	Compress_GZIP = Compress_Max + 1
)

// Compression values in the GraphStore batch write protocol.
type graphStoreCompression uint64

const (
	graphStoreCompressionNone graphStoreCompression = iota
	graphStoreCompressionDeflate
	graphStoreCompressionLZ4
	graphStoreCompressionZstd
)

type PostGraphStoreDataRequest struct {
	LogGroup     *LogGroup
	CompressType int // Zero defaults to Compress_LZ4.
}

type BatchPostGraphStoreDataRequest struct {
	LogGroups    []*LogGroup
	CompressType int // One compression type is applied independently to each group.
}

// PostGraphStoreData writes one nonempty LogGroup with one explicit __layout__ per Log.
// Success acknowledges acceptance. Retries can duplicate delivery.
func (c *Client) PostGraphStoreData(project, graphStore string, req *PostGraphStoreDataRequest) error {
	if req == nil {
		return NewClientError(fmt.Errorf("PostGraphStoreData request is required"))
	}
	groups := []*LogGroup{req.LogGroup}
	uri, compression, err := prepareGraphStoreWrite(project, graphStore, groups, req.CompressType)
	if err != nil {
		return err
	}
	raw, err := req.LogGroup.Marshal()
	if err != nil {
		return NewClientError(err)
	}
	body, err := compressGraphStoreGroup(raw, compression)
	if err != nil {
		return NewClientError(err)
	}
	headers := map[string]string{HTTPHeaderBodyRawSize: strconv.Itoa(len(raw))}
	switch compression {
	case graphStoreCompressionLZ4:
		headers["x-log-compresstype"] = "lz4"
	case graphStoreCompressionZstd:
		headers["x-log-compresstype"] = "zstd"
	case graphStoreCompressionDeflate:
		headers["x-log-compresstype"] = "deflate"
	}
	return c.sendGraphStoreWrite(project, uri, headers, body)
}

// BatchPostGraphStoreData writes nonempty LogGroups in a single batch_group request.
// Each Log requires one explicit __layout__, and each package carries its compression type.
func (c *Client) BatchPostGraphStoreData(project, graphStore string, req *BatchPostGraphStoreDataRequest) error {
	if req == nil {
		return NewClientError(fmt.Errorf("BatchPostGraphStoreData request is required"))
	}
	uri, compression, err := prepareGraphStoreWrite(project, graphStore, req.LogGroups, req.CompressType)
	if err != nil {
		return err
	}
	var body []byte
	for _, group := range req.LogGroups {
		raw, err := group.Marshal()
		if err != nil {
			return NewClientError(err)
		}
		data, err := compressGraphStoreGroup(raw, compression)
		if err != nil {
			return NewClientError(err)
		}
		body, err = appendGraphStorePackage(body, data, uint64(len(raw)), compression)
		if err != nil {
			return err
		}
	}
	headers := map[string]string{
		HTTPHeaderBodyRawSize: strconv.Itoa(len(body)),
		"x-log-mode":          "batch_group",
		"x-log-compresstype":  "mixed",
	}
	return c.sendGraphStoreWrite(project, uri, headers, body)
}

// The GraphStore package adds field 3 for explicit compression. Encode it here
// so the shared LogStore protobuf types keep their existing fields.
func appendGraphStorePackage(body, data []byte, rawSize uint64, compression graphStoreCompression) ([]byte, error) {
	const (
		packageField     = 1
		dataField        = 1
		rawSizeField     = 2
		compressionField = 3
	)
	packageSize := protowire.SizeTag(dataField) + protowire.SizeBytes(len(data)) +
		protowire.SizeTag(rawSizeField) + protowire.SizeVarint(rawSize) +
		protowire.SizeTag(compressionField) + protowire.SizeVarint(uint64(compression))
	if len(body)+protowire.SizeTag(packageField)+protowire.SizeBytes(packageSize) > GraphStoreMaxWriteBytes {
		return nil, graphStoreWriteSizeError()
	}
	body = protowire.AppendTag(body, packageField, protowire.BytesType)
	body = protowire.AppendVarint(body, uint64(packageSize))
	body = protowire.AppendTag(body, dataField, protowire.BytesType)
	body = protowire.AppendBytes(body, data)
	body = protowire.AppendTag(body, rawSizeField, protowire.VarintType)
	body = protowire.AppendVarint(body, rawSize)
	body = protowire.AppendTag(body, compressionField, protowire.VarintType)
	return protowire.AppendVarint(body, uint64(compression)), nil
}

func prepareGraphStoreWrite(project, graphStore string, groups []*LogGroup, compressType int) (string, graphStoreCompression, error) {
	var compression graphStoreCompression
	switch compressType {
	case Compress_None:
		compression = graphStoreCompressionNone
	case Compress_LZ4:
		compression = graphStoreCompressionLZ4
	case Compress_ZSTD:
		compression = graphStoreCompressionZstd
	case Compress_GZIP:
		compression = graphStoreCompressionDeflate
	default:
		return "", 0, InvalidCompressError
	}
	if project == "" {
		return "", 0, NewClientError(fmt.Errorf("project is required"))
	}
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return "", 0, NewClientError(err)
	}
	if len(groups) == 0 {
		return "", 0, NewClientError(fmt.Errorf("logGroups must not be empty"))
	}
	var records, rawSize int
	for i, group := range groups {
		if group == nil || len(group.Logs) == 0 {
			return "", 0, NewClientError(fmt.Errorf("logGroups[%d] must not be nil or empty", i))
		}
		records += len(group.Logs)
		if records > graphStoreMaxWriteLogs {
			return "", 0, NewClientError(fmt.Errorf("GraphStore write exceeds %d records", graphStoreMaxWriteLogs))
		}
		for j, record := range group.Logs {
			if err := validateGraphStoreRecord(record); err != nil {
				return "", 0, NewClientError(fmt.Errorf("logGroups[%d].logs[%d]: %w", i, j, err))
			}
		}
		for j, tag := range group.LogTags {
			if tag == nil || tag.Key == nil || tag.Value == nil {
				return "", 0, NewClientError(fmt.Errorf("logGroups[%d].logTags[%d] requires key and value", i, j))
			}
		}
		rawSize += group.Size()
		if rawSize > GraphStoreMaxWriteBytes {
			return "", 0, graphStoreWriteSizeError()
		}
	}
	return uri + "/write", compression, nil
}

func validateGraphStoreRecord(record *Log) error {
	if record == nil || record.GetTime() == 0 {
		return fmt.Errorf("nonzero protobuf Time is required")
	}
	hasLayout := false
	var from, to *string
	for i, field := range record.Contents {
		if field == nil || field.Key == nil || field.Value == nil {
			return fmt.Errorf("contents[%d] requires key and value", i)
		}
		switch field.GetKey() {
		case "__layout__":
			layout := GraphStoreLayoutType(field.GetValue())
			if hasLayout || (layout != GraphStoreLayoutVertex && layout != GraphStoreLayoutEdge) {
				return fmt.Errorf("__layout__ must occur once and be vertex or edge")
			}
			hasLayout = true
		case "__active_from__":
			from = field.Value
		case "__active_to__":
			to = field.Value
		}
	}
	if !hasLayout {
		return fmt.Errorf("__layout__ is required")
	}
	if from != nil || to != nil {
		if from == nil || !isGraphStoreWriteTime(*from) {
			return fmt.Errorf("__active_from__ requires a decimal uint32 Unix second")
		}
		if to == nil || !isGraphStoreWriteTime(*to) {
			return fmt.Errorf("__active_to__ requires a decimal uint32 Unix second")
		}
	}
	return nil
}

func isGraphStoreWriteTime(value string) bool {
	_, err := strconv.ParseUint(value, 10, 32)
	return err == nil
}

func compressGraphStoreGroup(raw []byte, compression graphStoreCompression) ([]byte, error) {
	switch compression {
	case graphStoreCompressionLZ4:
		out := make([]byte, lz4.CompressBlockBound(len(raw)))
		n, err := lz4.CompressBlock(raw, out, nil)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			n, _ = copyIncompressible(raw, out)
		}
		return out[:n], nil
	case graphStoreCompressionZstd:
		return slsZstdCompressor.Compress(raw, nil)
	case graphStoreCompressionDeflate:
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
		return raw, nil
	}
}

func graphStoreWriteSizeError() error {
	return &Error{HTTPCode: -1, Code: "PostBodyTooLarge", Message: "GraphStore request exceeds 10 MiB"}
}

func (c *Client) sendGraphStoreWrite(project, uri string, headers map[string]string, body []byte) error {
	if len(body) > GraphStoreMaxWriteBytes {
		return graphStoreWriteSizeError()
	}
	headers[HTTPHeaderContentType] = "application/x-protobuf"
	response, err := request(convert(c, project), http.MethodPost, uri, headers, body)
	if err != nil {
		return NewClientError(err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return readResponseError(err)
	}
	return nil
}
