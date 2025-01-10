package producer

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	uberatomic "go.uber.org/atomic"
)

type LogAccumulator struct {
	lock           sync.RWMutex
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

func (logAccumulator *LogAccumulator) addOrSendProducerBatch(key, project, logstore, logTopic, logSource, shardHash string, producerBatch *ProducerBatch, log interface{}, callback CallBack) {
	defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "addOrSendProducerBatch")
	totalDataCount := producerBatch.getLogGroupCount() + 1
	if int64(producerBatch.totalDataSize) > logAccumulator.producerConfig.MaxBatchSize && producerBatch.totalDataSize < 5242880 && totalDataCount <= logAccumulator.producerConfig.MaxBatchCount {
		producerBatch.addLogToLogGroup(log)
		if callback != nil {
			producerBatch.addProducerBatchCallBack(callback)
		}
		defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "innerSendToServer")
		logAccumulator.innerSendToServer(key, producerBatch)
	} else if int64(producerBatch.totalDataSize) <= logAccumulator.producerConfig.MaxBatchSize && totalDataCount <= logAccumulator.producerConfig.MaxBatchCount {
		defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "addOrSendProducerBatch2")
		producerBatch.addLogToLogGroup(log)
		if callback != nil {
			producerBatch.addProducerBatchCallBack(callback)
		}
	} else {
		defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "innerSendToServer2")
		logAccumulator.innerSendToServer(key, producerBatch)
		logAccumulator.createNewProducerBatch(log, callback, key, project, logstore, logTopic, logSource, shardHash)
	}
}

// In this function，Naming with mlog is to avoid conflicts with the introduced kit/log package names.
func (logAccumulator *LogAccumulator) addLogToProducerBatch(project, logstore, shardHash, logTopic, logSource string,
	logData interface{}, callback CallBack) error {
	if logAccumulator.shutDownFlag.Load() {
		level.Warn(logAccumulator.logger).Log("msg", "Producer has started and shut down and cannot write to new logs")
		return errors.New("Producer has started and shut down and cannot write to new logs")
	}
	defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "addLogToProducerBatch")
	key := logAccumulator.getKeyString(project, logstore, logTopic, shardHash, logSource)
	defer logAccumulator.lock.Unlock()
	logAccumulator.lock.Lock()
	if mlog, ok := logData.(*sls.Log); ok {
		if producerBatch, ok := logAccumulator.logGroupData[key]; ok == true {
			logSize := int64(GetLogSizeCalculate(mlog))
			atomic.AddInt64(&producerBatch.totalDataSize, logSize)
			atomic.AddInt64(&logAccumulator.producer.producerLogGroupSize, logSize)
			logAccumulator.addOrSendProducerBatch(key, project, logstore, logTopic, logSource, shardHash, producerBatch, mlog, callback)
		} else {
			logAccumulator.createNewProducerBatch(mlog, callback, key, project, logstore, logTopic, logSource, shardHash)
		}
	} else if logList, ok := logData.([]*sls.Log); ok {
		if producerBatch, ok := logAccumulator.logGroupData[key]; ok == true {
			logListSize := int64(GetLogListSize(logList))
			atomic.AddInt64(&producerBatch.totalDataSize, logListSize)
			atomic.AddInt64(&logAccumulator.producer.producerLogGroupSize, logListSize)
			logAccumulator.addOrSendProducerBatch(key, project, logstore, logTopic, logSource, shardHash, producerBatch, logList, callback)

		} else {
			logAccumulator.createNewProducerBatch(logList, callback, key, project, logstore, logTopic, logSource, shardHash)
		}
	} else {
		level.Error(logAccumulator.logger).Log("msg", "Invalid logType")
		return errors.New("Invalid logType")
	}
	return nil

}

func (logAccumulator *LogAccumulator) createNewProducerBatch(logType interface{}, callback CallBack, key, project, logstore, logTopic, logSource, shardHash string) {
	level.Debug(logAccumulator.logger).Log("msg", "Create a new ProducerBatch")
	defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "createNewProducerBatch")
	if mlog, ok := logType.(*sls.Log); ok {

		newProducerBatch := initProducerBatch(logAccumulator.packIdGenrator, mlog, callback, project, logstore, logTopic, logSource, shardHash, logAccumulator.producerConfig)
		logAccumulator.logGroupData[key] = newProducerBatch
	} else if logList, ok := logType.([]*sls.Log); ok {
		newProducerBatch := initProducerBatch(logAccumulator.packIdGenrator, logList, callback, project, logstore, logTopic, logSource, shardHash, logAccumulator.producerConfig)
		logAccumulator.logGroupData[key] = newProducerBatch
	}
}

func (logAccumulator *LogAccumulator) innerSendToServer(key string, producerBatch *ProducerBatch) {
	defer logAccumulator.producer.monitor.recordIfSlow(time.Now(), "innerSendToServerReal")
	level.Debug(logAccumulator.logger).Log("msg", "Send producerBatch to IoWorker from logAccumulator")
	logAccumulator.threadPool.addTask(producerBatch)
	delete(logAccumulator.logGroupData, key)
}

func (logAccumulator *LogAccumulator) getKeyString(project, logstore, logTopic, shardHash, logSource string) string {
	var key strings.Builder
	key.WriteString(project)
	key.WriteString(Delimiter)
	key.WriteString(logstore)
	key.WriteString(Delimiter)
	key.WriteString(logTopic)
	key.WriteString(Delimiter)
	key.WriteString(shardHash)
	key.WriteString(Delimiter)
	key.WriteString(logSource)
	return key.String()
}
