package producer

import (
	"net/http"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
)

const Delimiter = "|"

type UpdateStsTokenFunc = func() (accessKeyID, accessKeySecret, securityToken string, expireTime time.Time, err error)

type ProducerConfig struct {
	TotalSizeLnBytes    int64
	MaxIoWorkerCount    int64
	MaxBlockSec         int
	MaxBatchSize        int64
	MaxBatchCount       int
	LingerMs            int64
	Retries             int
	MaxReservedAttempts int
	BaseRetryBackoffMs  int64
	MaxRetryBackoffMs   int64
	AdjustShargHash     bool
	Buckets             int

	// Optional, defaults to nil.
	// The logger is used to record the runtime status of the consumer.
	// The logs generated by the logger will only be stored locally.
	// The parameters AllowLogLevel/LogFileName/LogMaxSize/LogMaxBackups/LogCompass
	// are ignored if the Logger is not nil.
	Logger log.Logger
	// Optional, defaults to info.
	// AllowLogLevel can be debug/info/warn/error, set the minimum level of the log to be recorded.
	AllowLogLevel string
	// Optional.
	// Setting Log File Path，eg: "/root/log/log_file.log". if not set, the log will go to stdout.
	LogFileName string
	// Optional, defaults to false.
	// Set whether the log output type is JSON.
	IsJsonType bool
	// Optional, defaults to 100, in megabytes.
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	LogMaxSize int
	// Optional, defaults to 10.
	// MaxBackups is the maximum number of old log files to retain.
	LogMaxBackups int
	// Optional, defaults to false.
	// Compress determines if the rotated log files should be compressed using gzip.
	LogCompress bool

	Endpoint              string
	NoRetryStatusCodeList []int
	HTTPClient            *http.Client
	UserAgent             string
	LogTags               []*sls.LogTag
	GeneratePackId        bool
	CredentialsProvider   sls.CredentialsProvider
	UseMetricStoreURL     bool
	DisableRuntimeMetrics bool // disable runtime metrics, runtime metrics prints to local log.

	// Deprecated: use CredentialsProvider and UpdateFuncProviderAdapter instead.
	//
	// Example:
	//   provider := sls.NewUpdateFuncProviderAdapter(updateStsTokenFunc)
	//   config := &ProducerConfig{
	//			CredentialsProvider: provider,
	//   }
	UpdateStsToken   UpdateStsTokenFunc
	StsTokenShutDown chan struct{}
	AccessKeyID      string // Deprecated: use CredentialsProvider instead
	AccessKeySecret  string // Deprecated: use CredentialsProvider instead
	Region           string
	AuthVersion      sls.AuthVersionType
	CompressType     int    // only work for logstore now
	Processor        string // ingest processor
}

func GetDefaultProducerConfig() *ProducerConfig {
	return &ProducerConfig{
		TotalSizeLnBytes:      100 * 1024 * 1024,
		MaxIoWorkerCount:      50,
		MaxBlockSec:           60,
		MaxBatchSize:          512 * 1024,
		LingerMs:              2000,
		Retries:               10,
		MaxReservedAttempts:   11,
		BaseRetryBackoffMs:    100,
		MaxRetryBackoffMs:     50 * 1000,
		AdjustShargHash:       true,
		Buckets:               64,
		MaxBatchCount:         4096,
		NoRetryStatusCodeList: []int{400, 404},
		CompressType:          sls.Compress_LZ4,
	}
}
