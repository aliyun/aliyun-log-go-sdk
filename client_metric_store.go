package sls

import "time"

// CreateMetricStore .
//
// Deprecated: use CreateMetricStoreV2 instead.
func (c *Client) CreateMetricStore(project string, metricStore *LogStore) error {
	metricStore.TelemetryType = "Metrics"
	err := c.CreateLogStoreV2(project, metricStore)
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 3)
	subStore := &SubStore{}
	subStore.Name = "prom"
	subStore.SortedKeyCount = 2
	subStore.TimeIndex = 2
	subStore.TTL = metricStore.TTL
	subStore.Keys = append(subStore.Keys, SubStoreKey{
		Name: "__name__",
		Type: "text",
	}, SubStoreKey{
		Name: "__labels__",
		Type: "text",
	}, SubStoreKey{
		Name: "__time_nano__",
		Type: "long",
	}, SubStoreKey{
		Name: "__value__",
		Type: "double",
	})
	if !subStore.IsValid() {
		panic("metric store invalid")
	}
	return c.CreateSubStore(project, metricStore.Name, subStore)
}

// UpdateMetricStore .
//
// Deprecated: use UpdateMetricStoreV2 instead.
func (c *Client) UpdateMetricStore(project string, metricStore *LogStore) error {
	metricStore.TelemetryType = "Metrics"
	err := c.UpdateLogStoreV2(project, metricStore)
	if err != nil {
		return err
	}
	return c.UpdateSubStoreTTL(project, metricStore.Name, metricStore.TTL)
}

// DeleteMetricStore .
//
// Deprecated: use DeleteMetricStoreV2 instead.
func (c *Client) DeleteMetricStore(project, name string) error {
	return c.DeleteLogStore(project, name)
}

// GetMetricStore .
//
// Deprecated: use GetMetricStoreV2 instead.
func (c *Client) GetMetricStore(project, name string) (*LogStore, error) {
	return c.GetLogStore(project, name)
}

// CreateMetricStoreV2 creates a new metric store in SLS.
func (c *Client) CreateMetricStoreV2(project string, metricStore *MetricStore) error {
	proj := convert(c, project)
	return proj.CreateMetricStoreV2(metricStore)
}

// UpdateMetricStoreV2 updates a metric store.
func (c *Client) UpdateMetricStoreV2(project string, metricStore *MetricStore) error {
	proj := convert(c, project)
	return proj.UpdateMetricStoreV2(metricStore)
}

// DeleteMetricStoreV2 deletes a metric store.
func (c *Client) DeleteMetricStoreV2(project, name string) error {
	proj := convert(c, project)
	return proj.DeleteMetricStoreV2(name)
}

// GetMetricStoreV2 returns a metric store.
func (c *Client) GetMetricStoreV2(project, name string) (*MetricStore, error) {
	proj := convert(c, project)
	return proj.GetMetricStoreV2(name)
}

// GetMetricStoreMeteringMode get the metering mode of metric store, eg. ChargeByFunction / ChargeByDataIngest
func (c *Client) GetMetricStoreMeteringMode(project string, metricStore string) (*GetMeteringModeResponse, error) {
	ms := convertLogstore(c, project, metricStore)
	return ms.GetMetricStoreMeteringMode()
}

// UpdateMetricStoreMeteringMode update the metering mode of metric store, eg. ChargeByFunction / ChargeByDataIngest
// Warning: this method may affect your billings, for more details ref: https://www.aliyun.com/price/detail/sls
func (c *Client) UpdateMetricStoreMeteringMode(project string, metricStore string, meteringMode string) error {
	ms := convertLogstore(c, project, metricStore)
	return ms.UpdateMetricStoreMeteringMode(meteringMode)
}
