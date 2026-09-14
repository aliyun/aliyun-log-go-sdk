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
	var sortedIndices []int
	if len(tags) > 0 {
		var stackIndices [16]int
		keySize += 1 + len(tags)*8
		for _, tag := range tags {
			keySize += len(tag.GetKey()) + len(tag.GetValue())
		}

		if len(tags) <= len(stackIndices) {
			order := logTagOrder{tags: tags, indices: stackIndices[:len(tags)]}
			for i := range order.indices {
				order.indices[i] = i
				if i == 0 || !order.Less(i, i-1) {
					continue
				}
				low, high := 0, i-1
				for low < high {
					mid := (low + high) / 2
					if order.Less(i, mid) {
						high = mid
					} else {
						low = mid + 1
					}
				}
				copy(order.indices[low+1:i+1], order.indices[low:i])
				order.indices[low] = i
			}
			sortedIndices = order.indices
		} else {
			// Keep the stack buffer out of sort.Interface's escape path.
			order := logTagOrder{tags: tags, indices: make([]int, len(tags))}
			for i := range order.indices {
				order.indices[i] = i
			}
			sort.Sort(&order)
			sortedIndices = order.indices
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
		for _, index := range sortedIndices {
			writeKeyField(&key, tags[index].GetKey())
			writeKeyField(&key, tags[index].GetValue())
		}
	}
	return key.String()
}

type logTagOrder struct {
	tags    []*sls.LogTag
	indices []int
}

func (order *logTagOrder) Len() int { return len(order.indices) }

func (order *logTagOrder) Less(i, j int) bool {
	left, right := order.tags[order.indices[i]], order.tags[order.indices[j]]
	return *left.Key < *right.Key || *left.Key == *right.Key && *left.Value < *right.Value
}

func (order *logTagOrder) Swap(i, j int) {
	order.indices[i], order.indices[j] = order.indices[j], order.indices[i]
}

func writeKeyField(key *strings.Builder, value string) {
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(value)))
	key.Write(length[:])
	key.WriteString(value)
}
