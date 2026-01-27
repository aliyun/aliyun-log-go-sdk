package sls

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/kit/log/level"
)

const (
	httpScheme  = "http://"
	httpsScheme = "https://"
	ipRegexStr  = `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}.*`
)

var (
	ipRegex = regexp.MustCompile(ipRegexStr)
)

// this file is deprecated and no maintenance
// see client_project.go

// DataRedundancyType
const (
	PROJECT_DATA_REDUNDANCY_TYPE_UNKNOWN = "Unknown"
	PROJECT_DATA_REDUNDANCY_TYPE_LRS     = "LRS"
	PROJECT_DATA_REDUNDANCY_TYPE_ZRS     = "ZRS"
)

// LogProject defines log project
type LogProject struct {
	Name               string `json:"projectName"`                  // Project name
	Description        string `json:"description"`                  // Project description
	Status             string `json:"status"`                       // Normal
	Owner              string `json:"owner"`                        // empty
	Region             string `json:"region"`                       // region id, eg cn-shanghai
	CreateTime         string `json:"createTime"`                   // unix time seconds, eg 1524539357
	LastModifyTime     string `json:"lastModifyTime"`               // unix time seconds, eg 1524539357
	DataRedundancyType string `json:"dataRedundancyType,omitempty"` // data redundancy type, valid values: ['LRS', 'ZRS']
	Location           string `json:"location,omitempty"`           // location, eg. cn-beijing-b

	Endpoint           string          // Deprecated: will be made private in the next version
	AccessKeyID        string          // Deprecated: will be made private in the next version
	AccessKeySecret    string          // Deprecated: will be made private in the next version
	SecurityToken      string          // Deprecated: will be made private in the next version
	UsingHTTP          bool            // Deprecated: will be made private in the next version
	UserAgent          string          // Deprecated: will be made private in the next version
	AuthVersion        AuthVersionType // Deprecated: will be made private in the next version
	baseURL            string
	retryTimeout       time.Duration
	httpClient         *http.Client
	credentialProvider CredentialsProvider

	// User defined common headers.
	//
	// When conflict with sdk pre-defined headers, the value will
	// be ignored
	commonHeaders map[string]string
	innerHeaders  map[string]string
}

// NewLogProject creates a new SLS project.
//
// Deprecated: use NewLogProjectV2 instead.
func NewLogProject(name, endpoint, accessKeyID, accessKeySecret string) (p *LogProject, err error) {
	p = &LogProject{
		Name:            name,
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		httpClient:      defaultHttpClient,
		retryTimeout:    defaultRetryTimeout,
	}
	p.parseEndpoint()
	return p, nil
}

// NewLogProjectV2 creates a new SLS project, with a CredentialsProvider.
func NewLogProjectV2(name, endpoint string, provider CredentialsProvider) (p *LogProject, err error) {
	p = &LogProject{
		Name:               name,
		Endpoint:           endpoint,
		httpClient:         defaultHttpClient,
		retryTimeout:       defaultRetryTimeout,
		credentialProvider: provider,
	}
	p.parseEndpoint()
	return p, nil
}

// Deprecated: With credentials provider
func (p *LogProject) WithCredentialsProvider(provider CredentialsProvider) *LogProject {
	p.credentialProvider = provider
	return p
}

// Deprecated: WithToken add token parameter
func (p *LogProject) WithToken(token string) (*LogProject, error) {
	p.SecurityToken = token
	return p, nil
}

// Deprecated: WithRequestTimeout with custom timeout for a request
func (p *LogProject) WithRequestTimeout(timeout time.Duration) *LogProject {
	if p.httpClient == defaultHttpClient || p.httpClient == nil {
		p.httpClient = newDefaultHTTPClient(timeout)
	} else {
		p.httpClient.Timeout = timeout
	}
	return p
}

// Deprecated: WithRetryTimeout with custom timeout for a operation
// each operation may send one or more HTTP requests in case of retry required.
func (p *LogProject) WithRetryTimeout(timeout time.Duration) *LogProject {
	p.retryTimeout = timeout
	return p
}

// RawRequest send raw http request to LogService and return the raw http response
// @note you should call http.Response.Body.Close() to close body stream
func (p *LogProject) RawRequest(method, uri string, headers map[string]string, body []byte) (*http.Response, error) {
	ctx := context.Background()
	return realRequest(ctx, p, method, uri, headers, body, DefaultRequestOption())
}

// ListLogStore returns all logstore names of project p.
func (p *LogProject) ListLogStore() ([]string, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}

	uri := "/logstores"
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Body struct {
		Count     int
		LogStores []string
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	storeNames := body.LogStores
	return storeNames, nil
}

// ListLogStoreV2 ...
func (p *LogProject) ListLogStoreV2(offset, size int, telemetryType string) ([]string, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}

	uri := fmt.Sprintf("/logstores?offset=%d&size=%d&telemetryType=%s", offset, size, telemetryType)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Body struct {
		Count     int
		LogStores []string
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	storeNames := body.LogStores
	return storeNames, nil
}

// GetLogStore returns logstore according by logstore name.
func (p *LogProject) GetLogStore(name string) (*LogStore, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}

	r, err := request(p, "GET", "/logstores/"+name, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	s := &LogStore{}
	err = json.Unmarshal(buf, s)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	s.Name = name
	s.project = p
	return s, nil
}

// CreateLogStore creates a new logstore in SLS,
// where name is logstore name,
// and ttl is time-to-live(in day) of logs,
// and shardCnt is the number of shards,
// and autoSplit is auto split,
// and maxSplitShard is the max number of shard.
func (p *LogProject) CreateLogStore(name string, ttl, shardCnt int, autoSplit bool, maxSplitShard int) error {
	type Body struct {
		Name          string `json:"logstoreName"`
		TTL           int    `json:"ttl"`
		ShardCount    int    `json:"shardCount"`
		AutoSplit     bool   `json:"autoSplit"`
		MaxSplitShard int    `json:"maxSplitShard"`
		WebTracking   bool   `json:"enable_tracking"`
	}
	store := &Body{
		Name:          name,
		TTL:           ttl,
		ShardCount:    shardCnt,
		AutoSplit:     autoSplit,
		MaxSplitShard: maxSplitShard,
	}
	body, err := json.Marshal(store)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}

	r, err := request(p, "POST", "/logstores", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// CreateLogStoreV2 creates a new logstore in SLS
func (p *LogProject) CreateLogStoreV2(logstore *LogStore) error {
	body, err := json.Marshal(logstore)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}

	r, err := request(p, "POST", "/logstores", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// DeleteLogStore deletes a logstore according by logstore name.
func (p *LogProject) DeleteLogStore(name string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}

	r, err := request(p, "DELETE", "/logstores/"+name, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// UpdateLogStore updates a logstore according by logstore name,
// obviously we can't modify the logstore name itself.
func (p *LogProject) UpdateLogStore(name string, ttl, shardCnt int) (err error) {
	type Body struct {
		Name       string `json:"logstoreName"`
		TTL        int    `json:"ttl"`
		ShardCount int    `json:"shardCount"`
	}
	store := &Body{
		Name:       name,
		TTL:        ttl,
		ShardCount: shardCnt,
	}
	body, err := json.Marshal(store)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/logstores/"+name, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// UpdateLogStoreV2 updates a logstore according by logstore name
// obviously we can't modify the logstore name itself.
func (p *LogProject) UpdateLogStoreV2(logstore *LogStore) (err error) {
	body, err := json.Marshal(logstore)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/logstores/"+logstore.Name, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// ListMachineGroup returns machine group name list and the total number of machine groups.
// The offset starts from 0 and the size is the max number of machine groups could be returned.
func (p *LogProject) ListMachineGroup(offset, size int) (m []string, total int, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	if size <= 0 {
		size = 500
	}
	uri := fmt.Sprintf("/machinegroups?offset=%v&size=%v", offset, size)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, 0, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, 0, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, 0, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Body struct {
		MachineGroups []string
		Count         int
		Total         int
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, 0, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	m = body.MachineGroups
	total = body.Total
	return m, total, nil
}

// CheckLogstoreExist check logstore exist or not
func (p *LogProject) CheckLogstoreExist(name string) (bool, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/logstores/"+name, h, nil)

	if err != nil {
		if _, ok := err.(*Error); ok {
			slsErr := err.(*Error)
			if slsErr.Code == "LogStoreNotExist" {
				return false, nil
			}
			return false, slsErr
		}
		return false, err
	}
	defer r.Body.Close()
	return true, nil
}

// CheckMachineGroupExist check machine group exist or not
func (p *LogProject) CheckMachineGroupExist(name string) (bool, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/machinegroups/"+name, h, nil)
	if err != nil {
		if _, ok := err.(*Error); ok {
			slsErr := err.(*Error)
			if slsErr.Code == "MachineGroupNotExist" {
				return false, nil
			}
			return false, slsErr
		}
		return false, err
	}
	defer r.Body.Close()
	return true, nil
}

// GetMachineGroup retruns machine group according by machine group name.
func (p *LogProject) GetMachineGroup(name string) (m *MachineGroup, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/machinegroups/"+name, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	m = new(MachineGroup)
	err = json.Unmarshal(buf, m)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	return m, nil
}

// CreateMachineGroup creates a new machine group in SLS.
func (p *LogProject) CreateMachineGroup(m *MachineGroup) error {
	body, err := json.Marshal(m)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "POST", "/machinegroups", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// UpdateMachineGroup updates a machine group.
func (p *LogProject) UpdateMachineGroup(m *MachineGroup) (err error) {
	body, err := json.Marshal(m)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/machinegroups/"+m.Name, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// DeleteMachineGroup deletes machine group according machine group name.
func (p *LogProject) DeleteMachineGroup(name string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "DELETE", "/machinegroups/"+name, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	return nil
}

func (p *LogProject) CreateMetricConfig(metricStore string, metricConfig *MetricsConfig) error {
	body, err := json.Marshal(metricConfig)
	if err != nil {
		return NewClientError(err)
	}
	jsonBody := map[string]interface{}{
		"metricStore":         metricStore,
		"metricsConfigDetail": string(body),
	}
	body, err = json.Marshal(jsonBody)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "POST", "/metricsconfigs", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	body, err = ioutil.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		err := new(Error)
		json.Unmarshal(body, err)
		return err
	}
	return nil
}

func (p *LogProject) DeleteMetricConfig(metricStore string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "DELETE", "/metricsconfigs/"+metricStore, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		err := new(Error)
		json.Unmarshal(body, err)
		return err
	}
	return nil
}

func (p *LogProject) GetMetricConfig(metricStore string) (*MetricsConfig, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/metricsconfigs/"+metricStore, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, _ := ioutil.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		err := new(Error)
		json.Unmarshal(buf, err)
		return nil, err
	}
	type OuterJSON struct {
		MetricStore         string `json:"metricStore"`
		MetricsConfigDetail string `json:"metricsConfigDetail"`
	}

	var outerData OuterJSON
	if err := json.Unmarshal(buf, &outerData); err != nil {
		log.Fatalf("Error parsing outer JSON: %v", err)
	}

	m := &MetricsConfig{}
	err = json.Unmarshal([]byte(outerData.MetricsConfigDetail), m)
	if err != nil {
		return nil, err
	}
	if IsDebugLevelMatched(4) {
		level.Info(Logger).Log("msg", "Get MetricConfig config, result", *m)
	}

	if reflect.DeepEqual(m, MetricsConfig{}) {
		fmt.Println("MetricsConfig is empty")
	}

	return m, err
}

func (p *LogProject) UpdateMetricConfig(metricStore string, metricConfig *MetricsConfig) (err error) {
	body, err := json.Marshal(metricConfig)
	if err != nil {
		return NewClientError(err)
	}
	jsonBody := map[string]interface{}{
		"metricStore":         metricStore,
		"metricsConfigDetail": string(body),
	}
	body, err = json.Marshal(jsonBody)
	if err != nil {
		return NewClientError(err)
	}
	body, err = json.Marshal(jsonBody)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/metricsconfigs/"+metricStore, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	body, _ = ioutil.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		err := new(Error)
		json.Unmarshal(body, err)
		return err
	}
	return nil
}

// ListConfig returns config names list and the total number of configs.
// The offset starts from 0 and the size is the max number of configs could be returned.
func (p *LogProject) ListConfig(offset, size int) (cfgNames []string, total int, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	if size <= 0 {
		size = 100
	}
	uri := fmt.Sprintf("/configs?offset=%v&size=%v", offset, size)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, 0, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, 0, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, 0, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Body struct {
		Total   int
		Configs []string
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, 0, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	cfgNames = body.Configs
	total = body.Total
	return cfgNames, total, nil
}

// CheckConfigExist check config exist or not
func (p *LogProject) CheckConfigExist(name string) (bool, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/configs/"+name, h, nil)
	if err != nil {
		if _, ok := err.(*Error); ok {
			slsErr := err.(*Error)
			if slsErr.Code == "ConfigNotExist" {
				return false, nil
			}
			return false, slsErr
		}
		return false, err
	}
	defer r.Body.Close()
	return true, nil
}

// GetConfig returns config according by config name.
func (p *LogProject) GetConfig(name string) (c *LogConfig, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/configs/"+name, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	c = &LogConfig{}
	err = json.Unmarshal(buf, c)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	if IsDebugLevelMatched(4) {
		level.Info(Logger).Log("msg", "Get logtail config, result", *c)
	}

	return c, nil
}

// UpdateConfig updates a config.
func (p *LogProject) UpdateConfig(c *LogConfig) (err error) {
	body, err := json.Marshal(c)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/configs/"+c.Name, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// CreateConfig creates a new config in SLS.
func (p *LogProject) CreateConfig(c *LogConfig) (err error) {
	body, err := json.Marshal(c)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "POST", "/configs", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// GetConfigString returns config according by config name.
func (p *LogProject) GetConfigString(name string) (c string, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", "/configs/"+name, h, nil)
	if err != nil {
		return "", NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return "", httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	if IsDebugLevelMatched(4) {
		level.Info(Logger).Log("msg", "Get logtail config, result", c)
	}
	return string(buf), err
}

// UpdateConfigString updates a config.
func (p *LogProject) UpdateConfigString(configName, c string) (err error) {
	body := []byte(c)

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "PUT", "/configs/"+configName, h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// CreateConfigString creates a new config in SLS.
func (p *LogProject) CreateConfigString(c string) (err error) {
	body := []byte(c)
	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate", // TODO: support lz4
	}
	r, err := request(p, "POST", "/configs", h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// DeleteConfig deletes a config according by config name.
func (p *LogProject) DeleteConfig(name string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "DELETE", "/configs/"+name, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	return nil
}

// GetAppliedMachineGroups returns applied machine group names list according config name.
func (p *LogProject) GetAppliedMachineGroups(confName string) (groupNames []string, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/configs/%v/machinegroups", confName)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Body struct {
		Count         int
		Machinegroups []string
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	groupNames = body.Machinegroups
	return groupNames, nil
}

// GetAppliedConfigs returns applied config names list according machine group name groupName.
func (p *LogProject) GetAppliedConfigs(groupName string) (confNames []string, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/machinegroups/%v/configs", groupName)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	type Cfg struct {
		Count   int      `json:"count"`
		Configs []string `json:"configs"`
	}
	body := &Cfg{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	confNames = body.Configs
	return confNames, nil
}

// ApplyConfigToMachineGroup applies config to machine group.
func (p *LogProject) ApplyConfigToMachineGroup(confName, groupName string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/machinegroups/%v/configs/%v", groupName, confName)
	r, err := request(p, "PUT", uri, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	return nil
}

// RemoveConfigFromMachineGroup removes config from machine group.
func (p *LogProject) RemoveConfigFromMachineGroup(confName, groupName string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/machinegroups/%v/configs/%v", groupName, confName)
	r, err := request(p, "DELETE", uri, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	return nil
}

func (p *LogProject) CreateEtlMeta(etlMeta *EtlMeta) (err error) {
	body, err := json.Marshal(etlMeta)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate",
	}
	r, err := request(p, "POST", fmt.Sprintf("/%v", EtlMetaURI), h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

func (p *LogProject) UpdateEtlMeta(etlMeta *EtlMeta) (err error) {
	body, err := json.Marshal(etlMeta)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate",
	}
	r, err := request(p, "PUT", fmt.Sprintf("/%v", EtlMetaURI), h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

func (p *LogProject) DeleteEtlMeta(etlMetaName, etlMetaKey string) (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/%v?etlMetaName=%v&etlMetaKey=%v&etlMetaTag=%v", EtlMetaURI, etlMetaName, etlMetaKey, EtlMetaAllTagMatch)
	r, err := request(p, "DELETE", uri, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

func (p *LogProject) listEtlMeta(etlMetaName, etlMetaKey, etlMetaTag string, offset, size int) (total int, count int, etlMeta []*EtlMeta, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/%v?offset=%v&size=%v&etlMetaName=%v&etlMetaKey=%v&etlMetaTag=%v", EtlMetaURI, offset, size, etlMetaName, etlMetaKey, etlMetaTag)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return 0, 0, nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, 0, nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return 0, 0, nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	type BodyMeta struct {
		MetaName  string `json:"etlMetaName"`
		MetaKey   string `json:"etlMetaKey"`
		MetaTag   string `json:"etlMetaTag"`
		MetaValue string `json:"etlMetaValue"`
	}
	type Body struct {
		Total    int         `json:"total"`
		Count    int         `json:"count"`
		MetaList []*BodyMeta `json:"etlMetaList"`
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return 0, 0, nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	if body.Count == 0 || len(body.MetaList) == 0 {
		return body.Total, body.Count, nil, nil
	}
	var etlMetaList []*EtlMeta = make([]*EtlMeta, len(body.MetaList))
	for index, value := range body.MetaList {
		var metaValueMap map[string]string
		err := json.Unmarshal([]byte(value.MetaValue), &metaValueMap)
		if err != nil {
			return 0, 0, nil, NewClientError(err)
		}
		etlMetaList[index] = &EtlMeta{
			MetaName:  value.MetaName,
			MetaKey:   value.MetaKey,
			MetaTag:   value.MetaTag,
			MetaValue: metaValueMap,
		}
	}
	return body.Total, body.Count, etlMetaList, nil
}

func (p *LogProject) GetEtlMeta(etlMetaName, etlMetaKey string) (etlMeta *EtlMeta, err error) {
	_, count, etlMetaList, err := p.listEtlMeta(etlMetaName, etlMetaKey, EtlMetaAllTagMatch, 0, 1)
	if err != nil {
		return nil, err
	} else if count == 0 {
		return nil, nil
	}
	return etlMetaList[0], nil
}

func (p *LogProject) ListEtlMeta(etlMetaName string, offset, size int) (total int, count int, etlMetaList []*EtlMeta, err error) {
	return p.listEtlMeta(etlMetaName, "", EtlMetaAllTagMatch, offset, size)
}

func (p *LogProject) ListEtlMetaWithTag(etlMetaName, etlMetaTag string, offset, size int) (total int, count int, etlMetaList []*EtlMeta, err error) {
	return p.listEtlMeta(etlMetaName, "", etlMetaTag, offset, size)
}

func (p *LogProject) ListEtlMetaName(offset, size int) (total int, count int, etlMetaNameList []string, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/%v?offset=%v&size=%v", EtlMetaNameURI, offset, size)
	r, err := request(p, "GET", uri, h, nil)
	if err != nil {
		return 0, 0, nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, 0, nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return 0, 0, nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	type Body struct {
		Total        int      `json:"total"`
		Count        int      `json:"count"`
		MetaNameList []string `json:"etlMetaNameList"`
	}
	body := &Body{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return 0, 0, nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	return body.Total, body.Count, body.MetaNameList, nil
}

func (p *LogProject) CreateLogging(detail *Logging) (err error) {
	body, err := json.Marshal(detail)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate",
	}
	r, err := request(p, "POST", fmt.Sprintf("/%v", LoggingURI), h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

func (p *LogProject) UpdateLogging(detail *Logging) (err error) {
	body, err := json.Marshal(detail)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%v", len(body)),
		"Content-Type":      "application/json",
		"Accept-Encoding":   "deflate",
	}
	r, err := request(p, "PUT", fmt.Sprintf("/%v", LoggingURI), h, body)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

func (p *LogProject) GetLogging() (c *Logging, err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	r, err := request(p, "GET", fmt.Sprintf("/%v", LoggingURI), h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	c = &Logging{}
	err = json.Unmarshal(buf, c)
	if err != nil {
		return nil, invalidJsonRespError(string(buf), r.Header, r.StatusCode)
	}
	if IsDebugLevelMatched(4) {
		level.Info(Logger).Log("msg", "Get logging, result", *c)
	}

	return c, nil
}

func (p *LogProject) DeleteLogging() (err error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}
	uri := fmt.Sprintf("/%v", LoggingURI)
	r, err := request(p, "DELETE", uri, h, nil)
	if err != nil {
		return NewClientError(err)
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return nil
}

// warning: call some method directly from Client lead to requestTimeout not working,
// we should fix it in the future by making breaking changes
func (p *LogProject) init() {
	p.parseEndpointIfNeeded()
	if p.retryTimeout == 0 {
		p.retryTimeout = defaultRetryTimeout
	}

	if p.httpClient == nil {
		p.httpClient = defaultHttpClient
	}
}

func (p *LogProject) getBaseURL() string {
	p.parseEndpointIfNeeded()
	return p.baseURL
}

func (p *LogProject) parseEndpointIfNeeded() {
	if len(p.baseURL) > 0 {
		return
	}
	p.parseEndpoint()
}

func (p *LogProject) parseEndpoint() {
	scheme := httpScheme // default to http scheme
	host := p.Endpoint

	if strings.HasPrefix(p.Endpoint, httpScheme) {
		scheme = httpScheme
		host = strings.TrimPrefix(p.Endpoint, scheme)
	} else if strings.HasPrefix(p.Endpoint, httpsScheme) {
		scheme = httpsScheme
		host = strings.TrimPrefix(p.Endpoint, scheme)
	}

	if GlobalForceUsingHTTP || p.UsingHTTP {
		scheme = httpScheme
	}
	if len(p.Name) == 0 {
		p.baseURL = fmt.Sprintf("%s%s", scheme, host)
	} else {
		p.baseURL = fmt.Sprintf("%s%s.%s", scheme, p.Name, host)
	}
}
