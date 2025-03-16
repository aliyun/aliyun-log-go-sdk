package sls

type MetricQueryCacheConfig struct {
	Enable bool `json:"enable"`
}

type MetricParallelConfig struct {
	Enable               bool   `json:"enable"`
	Mode                 string `json:"mode"`
	TimePieceInterval    int    `json:"time_piece_interval"`
	TimePieceCount       int    `json:"time_piece_count"`
	ParallelCountPerHost int    `json:"parallel_count_per_host"`
	TotalParallelCount   int    `json:"total_parallel_count"`
}

type MetricDownSamplingConfig struct {
	Base         MetricDownSamplingStatus   `json:"base"`
	Downsampling []MetricDownSamplingStatus `json:"downsampling"`
}

type MetricDownSamplingStatus struct {
	CreateTime        int64 `json:"create_time"`
	TTL               int   `json:"ttl"`
	ResolutionSeconds int   `json:"resolution_seconds"`
}

type MetricPushdownConfig struct {
	Enable bool `json:"enable"`
}

type MetricRemoteWriteConfig struct {
	Enable                 bool                   `json:"enable"`
	HistoryInterval        int                    `json:"history_interval"`
	FutureInterval         int                    `json:"future_interval"`
	ReplicaField           string                 `json:"replica_field"`
	ReplicaTimeoutSeconds  int                    `json:"replica_timeout_seconds"`
	ShardGroupStrategyList ShardGroupStrategyList `json:"shard_group_strategy_list"`
}

type ShardGroupStrategyList struct {
	Strategies     []ShardGroupStrategy `json:"strategies"`
	TryOtherShard  bool                 `json:"try_other_shard"`
	LastUpdateTime int                  `json:"last_update_time"`
}

type ShardGroupStrategy struct {
	MetricNames     []string `json:"metric_names"`
	HashLabels      []string `json:"hash_labels"`
	ShardGroupCount int      `json:"shard_group_count"`
	Priority        int      `json:"priority"`
}

type MetricStoreViewRoutingConfig struct {
	MetricNames   []string       `json:"metric_names"`
	ProjectStores []ProjectStore `json:"project_stores"`
}

type ProjectStore struct {
	ProjectName string `json:"project"`
	MetricStore string `json:"metricstore"`
}

type MetricsConfig struct {
	QueryCacheConfig        MetricQueryCacheConfig         `json:"query_cache_config"`
	ParallelConfig          MetricParallelConfig           `json:"parallel_config"`
	DownSamplingConfig      MetricDownSamplingConfig       `json:"downsampling_config"`
	PushdownConfig          MetricPushdownConfig           `json:"pushdown_config"`
	RemoteWriteConfig       MetricRemoteWriteConfig        `json:"remote_write_config"`
	StoreViewRoutingConfigs []MetricStoreViewRoutingConfig `json:"store_view_routing_config"`
}
