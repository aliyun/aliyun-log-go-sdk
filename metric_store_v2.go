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

	// EncryptConf holds the encryption config of the metric store.
	// Contract verified by e2e tests against the /metricstores API (note: this
	// API speaks camelCase, unlike the snake_case /logstores API):
	//   - create and update share the same semantics: once encryption is enabled,
	//     the config is immutable — an update carrying a different config is
	//     rejected (400 ParameterInvalid), while disabling (enable=false) or
	//     re-enabling with the identical config is allowed;
	//   - nil omits the field from the request, leaving the server-side
	//     encryption state untouched.
	EncryptConf *MetricStoreEncryptConf `json:"encryptConf,omitempty"`

	CreateTime     uint32 `json:"createTime,omitempty"`
	LastModifyTime uint32 `json:"lastModifyTime,omitempty"`
}

// MetricStoreEncryptConf is the encryption config for /metricstores APIs.
// It mirrors EncryptConf but with camelCase JSON keys: the /metricstores API
// silently ignores the snake_case keys (encrypt_conf/encrypt_type) used by
// the /logstores API, so reusing EncryptConf here would not work.
type MetricStoreEncryptConf struct {
	Enable      bool                           `json:"enable"`
	EncryptType string                         `json:"encryptType"` // e.g. "default", "m4"
	UserCmkInfo *MetricStoreEncryptUserCmkConf `json:"userCmkInfo,omitempty"`
}

// MetricStoreEncryptUserCmkConf is the BYOK (user CMK) config for metric
// stores, the camelCase counterpart of EncryptUserCmkConf.
type MetricStoreEncryptUserCmkConf struct {
	CmkKeyId string `json:"cmkKeyId"`
	Arn      string `json:"arn"`
	RegionId string `json:"regionId"`
}
