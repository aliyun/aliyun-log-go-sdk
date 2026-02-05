package sls

// MetricStore defines MetricStore struct for /metricstores APIs.
type MetricStore struct {
	Name          string `json:"name"`
	TTL           int    `json:"ttl"`
	ShardCount    int    `json:"shardCount"`
	WebTracking   bool   `json:"enable_tracking"`
	AutoSplit     bool   `json:"autoSplit"`
	MaxSplitShard int    `json:"maxSplitShard"`

	AppendMeta          bool   `json:"appendMeta"`
	HotTTL              int32  `json:"hot_ttl,omitempty"`             // 0 means hot_ttl = ttl
	InfrequentAccessTTL *int32 `json:"infrequentAccessTTL,omitempty"` // 0 means infrequentAccessTTL = 0
	Mode                string `json:"mode,omitempty"`                // "query" or "standard"(default), can't be modified after creation

	CreateTime     uint32 `json:"createTime,omitempty"`
	LastModifyTime uint32 `json:"lastModifyTime,omitempty"`
}
