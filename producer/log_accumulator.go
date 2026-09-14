package producer

import (
	"encoding/binary"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	uberatomic "go.uber.org/atomic"
)

type LogAccumulator struct {
	lock           sync.Mutex
	logGroupData   map[string]*ProducerBatch
	producerConfig *ProducerConfig
	ioWorker       *IoWorker
	shutDownFlag   *uberatomic.Bool
	logger         log.Logger
	threadPool     *IoThreadPool
	producer       *Producer
	packIdGenrator *PackIdGenerator
}

func initLogAccumulator(config *ProducerConfig, ioWorker *IoWorker, logger log.Logger, threadPool *IoThreadPool, producer *Producer) *LogAccumulator {
	return &LogAccumulator{
		logGroupData:   make(map[string]*ProducerBatch),
		producerConfig: config,
		ioWorker:       ioWorker,
		shutDownFlag:   uberatomic.NewBool(false),
		logger:         logger,
		threadPool:     threadPool,
		producer:       producer,
		packIdGenrator: newPackIdGenerator(),
	}
}

func (logAccumulator *LogAccumulator) addLogToProducerBatch(project, logstore, shardHash, logTopic, logSource string,
	logData interface{}, tags []*sls.LogTag, callback CallBack) error {
	if logAccumulator.shutDownFlag.Load() {
		level.Warn(logAccumulator.logger).Log("msg", "Producer has started and shut down and cannot write to new logs")
		return errors.New("Producer has started and shut down and cannot write to new logs")
	}
	if log, ok := logData.(*sls.Log); ok {
		logAccumulator.addLog(project, logstore, shardHash, logTopic, logSource, log, callback)
		return nil
	}
	if logList, ok := logData.([]*sls.Log); ok {
		for _, tag := range tags {
			if tag == nil || tag.Key == nil || tag.Value == nil {
				return errors.New("log tag must have a key and value")
			}
		}
		logAccumulator.addLogList(project, logstore, shardHash, logTopic, logSource, logList, tags, callback)
		return nil
	}
	level.Error(logAccumulator.logger).Log("msg", "Invalid logType")
	return errors.New("invalid logType")
}

func (logAccumulator *LogAccumulator) addLog(project, logstore, shardHash, logTopic, logSource string,
	log *sls.Log, callback CallBack) {
	key := logAccumulator.getKeyString(project, logstore, logTopic, shardHash, logSource, nil)
	logSize := int64(GetLogSizeCalculate(log))
	atomic.AddInt64(&logAccumulator.producer.producerLogGroupSize, logSize)

	logAccumulator.lock.Lock()
	producerBatch := logAccumulator.getOrCreateProducerBatch(key, project, logstore, logTopic, logSource, shardHash, nil)
	producerBatch.addLog(log, logSize, callback)

	if !producerBatch.meetSendCondition(logAccumulator.producerConfig) {
		logAccumulator.lock.Unlock()
		return
	}

	logAccumulator.logGroupData[key] = nil
	logAccumulator.lock.Unlock()

	logAccumulator.threadPool.addTask(producerBatch)
}

func (logAccumulator *LogAccumulator) addLogList(project, logstore, shardHash, logTopic, logSource string,
	logList []*sls.Log, tags []*sls.LogTag, callback CallBack) {
	key := logAccumulator.getKeyString(project, logstore, logTopic, shardHash, logSource, tags)
	logListSize := int64(GetLogListSize(logList))
	atomic.AddInt64(&logAccumulator.producer.producerLogGroupSize, logListSize)

	logAccumulator.lock.Lock()
	producerBatch := logAccumulator.getOrCreateProducerBatch(key, project, logstore, logTopic, logSource, shardHash, tags)
	producerBatch.addLogList(logList, logListSize, callback)

	if !producerBatch.meetSendCondition(logAccumulator.producerConfig) {
		logAccumulator.lock.Unlock()
		return
	}

	logAccumulator.logGroupData[key] = nil
	logAccumulator.lock.Unlock()

	logAccumulator.threadPool.addTask(producerBatch)
}

func (logAccumulator *LogAccumulator) getOrCreateProducerBatch(key, project, logstore, logTopic, logSource, shardHash string, tags []*sls.LogTag) *ProducerBatch {
	if producerBatch, ok := logAccumulator.logGroupData[key]; ok && producerBatch != nil {
		return producerBatch
	}

	logAccumulator.producer.monitor.incCreateBatch()
	batch := newProducerBatch(logAccumulator.packIdGenrator, project, logstore, logTopic, logSource, shardHash, tags, logAccumulator.producerConfig)
	logAccumulator.logGroupData[key] = batch
	return batch
}

func (logAccumulator *LogAccumulator) getKeyString(project, logstore, logTopic, shardHash, logSource string, tags []*sls.LogTag) string {
	keySize := len(project) + len(logstore) + len(logTopic) + len(shardHash) + len(logSource) + 4
	sortedTags := tags
	if len(tags) > 0 {
		keySize += 1 + len(tags)*8
		for _, tag := range tags {
			keySize += len(tag.GetKey()) + len(tag.GetValue())
		}
		if len(tags) > 1 {
			sortedTags = make([]*sls.LogTag, len(tags))
			copy(sortedTags, tags)
			sort.Sort(logTagsByKeyValue(sortedTags))
		}
	}

	// Routing fields are assumed not to contain '$'; tag fields remain length-prefixed.
	var key strings.Builder
	key.Grow(keySize)
	key.WriteString(project)
	key.WriteByte('$')
	key.WriteString(logstore)
	key.WriteByte('$')
	key.WriteString(logTopic)
	key.WriteByte('$')
	key.WriteString(shardHash)
	key.WriteByte('$')
	key.WriteString(logSource)
	if len(tags) > 0 {
		key.WriteByte('$')
		for _, tag := range sortedTags {
			writeKeyField(&key, tag.GetKey())
			writeKeyField(&key, tag.GetValue())
		}
	}
	return key.String()
}

type logTagsByKeyValue []*sls.LogTag

func (tags logTagsByKeyValue) Len() int { return len(tags) }

func (tags logTagsByKeyValue) Less(i, j int) bool {
	left, right := tags[i], tags[j]
	return *left.Key < *right.Key || *left.Key == *right.Key && *left.Value < *right.Value
}

func (tags logTagsByKeyValue) Swap(i, j int) { tags[i], tags[j] = tags[j], tags[i] }

func writeKeyField(key *strings.Builder, value string) {
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(value)))
	key.Write(length[:])
	key.WriteString(value)
}
