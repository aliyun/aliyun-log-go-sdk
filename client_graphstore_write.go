package sls

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

const (
	// GraphStoreMaxWriteBytes bounds both uncompressed input and the HTTP request body.
	GraphStoreMaxWriteBytes = 10 * 1024 * 1024
	graphStoreMaxWriteLogs  = 100000
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
	body, err := compression.compress(raw)
	if err != nil {
		return NewClientError(err)
	}
	headers := logWriteHeaders(len(raw), compression.header)
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
		data, err := compression.compress(raw)
		if err != nil {
			return NewClientError(err)
		}
		body, err = appendGraphStorePackage(body, data, uint64(len(raw)), compression.packageValue)
		if err != nil {
			return err
		}
	}
	headers := logWriteHeaders(len(body), "mixed")
	headers["x-log-mode"] = "batch_group"
	return c.sendGraphStoreWrite(project, uri, headers, body)
}

// The SLS batch protocol carries explicit compression in field 3. Encode it
// here so the shared Go protobuf types keep their existing fields.
func appendGraphStorePackage(body, data []byte, rawSize, compression uint64) ([]byte, error) {
	const (
		packageField     = 1
		dataField        = 1
		rawSizeField     = 2
		compressionField = 3
	)
	packageSize := protowire.SizeTag(dataField) + protowire.SizeBytes(len(data)) +
		protowire.SizeTag(rawSizeField) + protowire.SizeVarint(rawSize) +
		protowire.SizeTag(compressionField) + protowire.SizeVarint(compression)
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
	return protowire.AppendVarint(body, compression), nil
}

func prepareGraphStoreWrite(project, graphStore string, groups []*LogGroup, compressType int) (string, logWriteCompression, error) {
	compression, err := resolveLogWriteCompression(compressType)
	if err != nil {
		return "", logWriteCompression{}, err
	}
	if project == "" {
		return "", logWriteCompression{}, NewClientError(fmt.Errorf("project is required"))
	}
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return "", logWriteCompression{}, NewClientError(err)
	}
	if len(groups) == 0 {
		return "", logWriteCompression{}, NewClientError(fmt.Errorf("logGroups must not be empty"))
	}
	var records, rawSize int
	for i, group := range groups {
		if group == nil || len(group.Logs) == 0 {
			return "", logWriteCompression{}, NewClientError(fmt.Errorf("logGroups[%d] must not be nil or empty", i))
		}
		records += len(group.Logs)
		if records > graphStoreMaxWriteLogs {
			return "", logWriteCompression{}, NewClientError(fmt.Errorf("GraphStore write exceeds %d records", graphStoreMaxWriteLogs))
		}
		for j, record := range group.Logs {
			if err := validateGraphStoreRecord(record); err != nil {
				return "", logWriteCompression{}, NewClientError(fmt.Errorf("logGroups[%d].logs[%d]: %w", i, j, err))
			}
		}
		for j, tag := range group.LogTags {
			if tag == nil || tag.Key == nil || tag.Value == nil {
				return "", logWriteCompression{}, NewClientError(fmt.Errorf("logGroups[%d].logTags[%d] requires key and value", i, j))
			}
		}
		rawSize += group.Size()
		if rawSize > GraphStoreMaxWriteBytes {
			return "", logWriteCompression{}, graphStoreWriteSizeError()
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

func graphStoreWriteSizeError() error {
	return &Error{HTTPCode: -1, Code: "PostBodyTooLarge", Message: "GraphStore request exceeds 10 MiB"}
}

func (c *Client) sendGraphStoreWrite(project, uri string, headers map[string]string, body []byte) error {
	if len(body) > GraphStoreMaxWriteBytes {
		return graphStoreWriteSizeError()
	}
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
