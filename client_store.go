package sls

import (
	base64E "encoding/base64"
	"fmt"
	"strconv"
	"time"
)

func convertLogstore(c *Client, project, logstore string) *LogStore {
	c.accessKeyLock.RLock()
	proj := convertLocked(c, project)
	c.accessKeyLock.RUnlock()
	return &LogStore{
		project: proj,
		Name:    logstore,
	}
}

// ListShards returns shard id list of this logstore.
func (c *Client) ListShards(project, logstore string) (shardIDs []*Shard, err error) {
	ls := convertLogstore(c, project, logstore)
	return ls.ListShards()
}

// SplitShard https://help.aliyun.com/document_detail/29021.html
func (c *Client) SplitShard(project, logstore string, shardID int, splitKey string) (shards []*Shard, err error) {
	return c.splitShard(project, logstore, shardID, 0, splitKey)
}

// SplitNumShard https://help.aliyun.com/document_detail/29021.html
func (c *Client) SplitNumShard(project, logstore string, shardID, shardsNum int) (shards []*Shard, err error) {
	return c.splitShard(project, logstore, shardID, shardsNum, "")
}

func (c *Client) splitShard(project, logstore string, shardID, shardsNum int, splitKey string) ([]*Shard, error) {
	queryParams := map[string]string{
		"action": "split",
	}
	if splitKey != "" {
		queryParams["key"] = splitKey
	}
	if shardsNum > 0 {
		queryParams["shardCount"] = strconv.Itoa(shardsNum)
	}
	path := fmt.Sprintf("/logstores/%v/shards/%v", logstore, shardID)
	var shards []*Shard
	if err := c.doRequest(project, "POST", path, queryParams, nil, nil, &shards); err != nil {
		return nil, err
	}
	return shards, nil
}

// MergeShards https://help.aliyun.com/document_detail/29022.html
func (c *Client) MergeShards(project, logstore string, shardID int) ([]*Shard, error) {
	var shards []*Shard
	uri := fmt.Sprintf("/logstores/%v/shards/%v", logstore, shardID)
	if err := c.doRequest(project, "POST", uri, map[string]string{
		"action": "merge",
	}, nil, nil, &shards); err != nil {
		return nil, err
	}
	return shards, nil
}

// PutLogs put logs into logstore.
// The callers should transform user logs into LogGroup.
func (c *Client) PutLogs(project, logstore string, lg *LogGroup) (err error) {
	ls := convertLogstore(c, project, logstore)
	return ls.PutLogs(lg)
}

// PostLogStoreLogs put logs into Shard logstore by hashKey.
// The callers should transform user logs into LogGroup.
func (c *Client) PostLogStoreLogs(project, logstore string, lg *LogGroup, hashKey *string) (err error) {
	ls := convertLogstore(c, project, logstore)
	req := &PostLogStoreLogsRequest{
		LogGroup: lg,
		HashKey:  hashKey,
	}
	return ls.PostLogStoreLogs(req)
}

func (c *Client) PutLogsWithMetricStoreURL(project, logstore string, lg *LogGroup) (err error) {
	ls := convertLogstore(c, project, logstore)
	ls.useMetricStoreURL = true
	return ls.PutLogs(lg)
}

func (c *Client) PostLogStoreLogsV2(project, logstore string, req *PostLogStoreLogsRequest) (err error) {
	ls := convertLogstore(c, project, logstore)
	return ls.PostLogStoreLogs(req)
}

// PostRawLogWithCompressType put raw log data to log service, no marshal
func (c *Client) PostRawLogWithCompressType(project, logstore string, rawLogData []byte, compressType int, hashKey *string) (err error) {
	ls := convertLogstore(c, project, logstore)
	if err := ls.SetPutLogCompressType(compressType); err != nil {
		return err
	}
	return ls.PostRawLogs(rawLogData, hashKey)
}

// PutLogsWithCompressType put logs into logstore with specific compress type.
// The callers should transform user logs into LogGroup.
func (c *Client) PutLogsWithCompressType(project, logstore string, lg *LogGroup, compressType int) (err error) {
	ls := convertLogstore(c, project, logstore)
	if err := ls.SetPutLogCompressType(compressType); err != nil {
		return err
	}
	return ls.PutLogs(lg)
}

// PutRawLogWithCompressType put raw log data to log service, no marshal
func (c *Client) PutRawLogWithCompressType(project, logstore string, rawLogData []byte, compressType int) (err error) {
	ls := convertLogstore(c, project, logstore)
	if err := ls.SetPutLogCompressType(compressType); err != nil {
		return err
	}
	return ls.PutRawLog(rawLogData)
}

// GetCursor gets log cursor of one shard specified by shardId.
// The from can be in three form: a) unix timestamp in seccond, b) "begin", c) "end".
// For more detail please read: https://help.aliyun.com/document_detail/29024.html
func (c *Client) GetCursor(project, logstore string, shardID int, from string) (cursor string, err error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetCursor(shardID, from)
}

// GetCursorTime ...
func (c *Client) GetCursorTime(project, logstore string, shardID int, cursor string) (cursorTime time.Time, err error) {
	path := fmt.Sprintf("/logstores/%v/shards/%v", logstore, shardID)
	type getCursorResult struct {
		CursorTime int `json:"cursor_time"`
	}
	var rst getCursorResult
	if err := c.doRequest(project, "GET", path, map[string]string{
		"cursor": cursor,
		"type":   "cursor_time",
	}, nil, nil, &rst); err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(rst.CursorTime), 0), nil
}

// GetPrevCursorTime ...
func (c *Client) GetPrevCursorTime(project, logstore string, shardID int, cursor string) (cursorTime time.Time, err error) {
	realCursor, err := base64E.StdEncoding.DecodeString(cursor)
	if err != nil {
		return cursorTime, NewClientError(err)
	}
	cursorVal, err := strconv.Atoi(string(realCursor))
	if err != nil {
		return cursorTime, NewClientError(err)
	}
	cursorVal--
	nextCursor := base64E.StdEncoding.EncodeToString([]byte(strconv.Itoa(cursorVal)))
	return c.GetCursorTime(project, logstore, shardID, nextCursor)
}

// GetLogsBytes gets logs binary data from shard specified by shardId according cursor and endCursor.
// The logGroupMaxCount is the max number of logGroup could be returned.
// The nextCursor is the next curosr can be used to read logs at next time.
func (c *Client) GetLogsBytes(project, logstore string, shardID int, cursor, endCursor string,
	logGroupMaxCount int) (out []byte, nextCursor string, err error) {
	plr := &PullLogRequest{
		Project:          project,
		Logstore:         logstore,
		ShardID:          shardID,
		Cursor:           cursor,
		EndCursor:        endCursor,
		LogGroupMaxCount: logGroupMaxCount,
	}
	return c.GetLogsBytesV2(plr)
}

func (c *Client) GetLogsBytesV2(plr *PullLogRequest) (out []byte, nextCursor string, err error) {
	ls := convertLogstore(c, plr.Project, plr.Logstore)
	return ls.GetLogsBytesV2(plr)
}

func (c *Client) GetLogsBytesWithQuery(plr *PullLogRequest) (out []byte, plm *PullLogMeta, err error) {
	ls := convertLogstore(c, plr.Project, plr.Logstore)
	return ls.GetLogsBytesWithQuery(plr)
}

// PullLogs gets logs from shard specified by shardId according cursor and endCursor.
// The logGroupMaxCount is the max number of logGroup could be returned.
// The nextCursor is the next cursor can be used to read logs at next time.
// @note if you want to pull logs continuous, set endCursor = ""
func (c *Client) PullLogs(project, logstore string, shardID int, cursor, endCursor string,
	logGroupMaxCount int) (gl *LogGroupList, nextCursor string, err error) {
	ls := convertLogstore(c, project, logstore)
	return ls.PullLogs(shardID, cursor, endCursor, logGroupMaxCount)
}

func (c *Client) PullLogsV2(plr *PullLogRequest) (gl *LogGroupList, nextCursor string, err error) {
	ls := convertLogstore(c, plr.Project, plr.Logstore)
	return ls.PullLogsV2(plr)
}

func (c *Client) PullLogsWithQuery(plr *PullLogRequest) (gl *LogGroupList, plm *PullLogMeta, err error) {
	ls := convertLogstore(c, plr.Project, plr.Logstore)
	return ls.PullLogsWithQuery(plr)
}

// GetHistograms query logs with [from, to) time range
func (c *Client) GetHistograms(project, logstore string, topic string, from int64, to int64, queryExp string) (*GetHistogramsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetHistograms(topic, from, to, queryExp)
}

func (c *Client) GetHistogramsV2(project, logstore string, ghr *GetHistogramRequest) (*GetHistogramsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetHistogramsV2(ghr)
}

// GetHistogramsToCompleted query logs with [from, to) time range to completed
func (c *Client) GetHistogramsToCompleted(project, logstore string, topic string, from int64, to int64, queryExp string) (*GetHistogramsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetHistogramsToCompleted(topic, from, to, queryExp)
}

func (c *Client) GetHistogramsToCompletedV2(project, logstore string, ghr *GetHistogramRequest) (*GetHistogramsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetHistogramsToCompletedV2(ghr)
}

// GetLogs query logs with [from, to) time range
func (c *Client) GetLogs(project, logstore string, topic string, from int64, to int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*GetLogsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogs(topic, from, to, queryExp, maxLineNum, offset, reverse)
}

func (c *Client) GetLogsByNano(project, logstore string, topic string, fromInNs int64, toInNs int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*GetLogsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsByNano(topic, fromInNs, toInNs, queryExp, maxLineNum, offset, reverse)
}

// GetLogsToCompleted query logs with [from, to) time range to completed
func (c *Client) GetLogsToCompleted(project, logstore string, topic string, from int64, to int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*GetLogsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsToCompleted(topic, from, to, queryExp, maxLineNum, offset, reverse)
}

// GetLogLines ...
func (c *Client) GetLogLines(project, logstore string, topic string, from int64, to int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*GetLogLinesResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogLines(topic, from, to, queryExp, maxLineNum, offset, reverse)
}

func (c *Client) GetLogLinesByNano(project, logstore string, topic string, fromInNs int64, toInNs int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*GetLogLinesResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogLinesByNano(topic, fromInNs, toInNs, queryExp, maxLineNum, offset, reverse)
}

// GetLogsV2 ...
func (c *Client) GetLogsV2(project, logstore string, req *GetLogRequest) (*GetLogsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsV2(req)
}

// GetLogsV3 ...
func (c *Client) GetLogsV3(project, logstore string, req *GetLogRequest) (*GetLogsV3Response, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsV3(req)
}

// GetLogsToCompletedV2 ...
func (c *Client) GetLogsToCompletedV2(project, logstore string, req *GetLogRequest) (*GetLogsResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsToCompletedV2(req)
}

// GetLogsToCompletedV3 ...
func (c *Client) GetLogsToCompletedV3(project, logstore string, req *GetLogRequest) (*GetLogsV3Response, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogsToCompletedV3(req)
}

// GetLogLinesV2 ...
func (c *Client) GetLogLinesV2(project, logstore string, req *GetLogRequest) (*GetLogLinesResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetLogLinesV2(req)
}

// CreateIndex ...
func (c *Client) CreateIndex(project, logstore string, index Index) error {
	ls := convertLogstore(c, project, logstore)
	return ls.CreateIndex(index)
}

// UpdateIndex ...
func (c *Client) UpdateIndex(project, logstore string, index Index) error {
	ls := convertLogstore(c, project, logstore)
	return ls.UpdateIndex(index)
}

// GetIndex ...
func (c *Client) GetIndex(project, logstore string) (*Index, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetIndex()
}

// CreateIndexString ...
func (c *Client) CreateIndexString(project, logstore string, index string) error {
	ls := convertLogstore(c, project, logstore)
	return ls.CreateIndexString(index)
}

// UpdateIndexString ...
func (c *Client) UpdateIndexString(project, logstore string, index string) error {
	ls := convertLogstore(c, project, logstore)
	return ls.UpdateIndexString(index)
}

// GetIndexString ...
func (c *Client) GetIndexString(project, logstore string) (string, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetIndexString()
}

// DeleteIndex ...
func (c *Client) DeleteIndex(project, logstore string) error {
	ls := convertLogstore(c, project, logstore)
	return ls.DeleteIndex()
}

// ListSubStore ...
func (c *Client) ListSubStore(project, logstore string) (subStoreNames []string, err error) {
	path := fmt.Sprintf("/logstores/%v/substores", logstore)
	type sortedSubStoreList struct {
		SubStores []string `json:"substores"`
	}
	var body sortedSubStoreList
	if err := c.doRequest(project, "GET", path, nil, nil, nil, &body); err != nil {
		return nil, err
	}
	return body.SubStores, nil
}

// GetSubStore ...
func (c *Client) GetSubStore(project, logstore, name string) (*SubStore, error) {
	path := fmt.Sprintf("/logstores/%s/substores/%s", logstore, name)
	var sortedSubStore SubStore
	if err := c.doRequest(project, "GET", path, nil, nil, nil, &sortedSubStore); err != nil {
		return nil, err
	}
	return &sortedSubStore, nil
}

// CreateSubStore ...
func (c *Client) CreateSubStore(project, logstore string, sss *SubStore) error {
	path := fmt.Sprintf("/logstores/%s/substores", logstore)
	return c.doRequest(project, "POST", path, nil, nil, sss, nil)
}

// UpdateSubStore ...
func (c *Client) UpdateSubStore(project, logstore string, sss *SubStore) error {
	path := fmt.Sprintf("/logstores/%s/substores/%s", logstore, sss.Name)
	return c.doRequest(project, "PUT", path, nil, nil, sss, nil)
}

// DeleteSubStore ...
func (c *Client) DeleteSubStore(project, logstore string, name string) error {
	path := fmt.Sprintf("/logstores/%s/substores/%s", logstore, name)
	return c.doRequest(project, "DELETE", path, nil, nil, nil, nil)
}

// GetSubStoreTTL ...
func (c *Client) GetSubStoreTTL(project, logstore string) (ttl int, err error) {
	path := fmt.Sprintf("/logstores/%s/substores/storage/ttl", logstore)
	type ttlDef struct {
		TTL int `json:"ttl"`
	}
	var ttlIns ttlDef
	if err := c.doRequest(project, "GET", path, nil, nil, nil, &ttlIns); err != nil {
		return 0, err
	}
	return ttlIns.TTL, nil
}

// UpdateSubStoreTTL ...
func (c *Client) UpdateSubStoreTTL(project, logstore string, ttl int) error {
	path := fmt.Sprintf("/logstores/%s/substores/storage/ttl", logstore)
	return c.doRequest(project, "PUT", path, map[string]string{
		"ttl": strconv.Itoa(ttl),
	}, nil, nil, nil)
}
