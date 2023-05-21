package consumerLibrary

import (
	"sync"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type ShardConsumerWorker struct {
	client                    *ConsumerClient
	consumerCheckPointTracker *DefaultCheckPointTracker
	shutdownFlag              bool
	lastFetchLogGroupList     *sls.LogGroupList
	nextFetchCursor           string
	lastFetchGroupCount       int
	lastFetchTime             time.Time
	consumerStatus            string
	process                   func(shard int, logGroup *sls.LogGroupList, checkpointTracker CheckPointTracker) string
	shardId                   int
	// TODO: refine to channel
	isCurrentDone bool
	logger        log.Logger
	// unix time
	lastCheckpointSaveTime time.Time
	rollBackCheckpoint     string

	taskLock   sync.RWMutex
	statusLock sync.RWMutex
}

func (consumer *ShardConsumerWorker) setConsumerStatus(status string) {
	consumer.statusLock.Lock()
	defer consumer.statusLock.Unlock()
	consumer.consumerStatus = status
}

func (consumer *ShardConsumerWorker) getConsumerStatus() string {
	consumer.statusLock.RLock()
	defer consumer.statusLock.RUnlock()
	return consumer.consumerStatus
}

func initShardConsumerWorker(shardId int, consumerClient *ConsumerClient, consumerHeartBeat *ConsumerHeartBeat, do func(shard int, logGroup *sls.LogGroupList, checkpointTracker CheckPointTracker) string, logger log.Logger) *ShardConsumerWorker {
	shardConsumeWorker := &ShardConsumerWorker{
		shutdownFlag:              false,
		process:                   do,
		consumerCheckPointTracker: initConsumerCheckpointTracker(shardId, consumerClient, consumerHeartBeat, logger),
		client:                    consumerClient,
		consumerStatus:            INITIALIZING,
		shardId:                   shardId,
		lastFetchTime:             time.Now(),
		isCurrentDone:             true,
		logger:                    logger,
		rollBackCheckpoint:        "",
	}
	return shardConsumeWorker
}

func (consumer *ShardConsumerWorker) consume() {
	if !consumer.isTaskDone() {
		return
	}

	// start a new task
	// initial task / fetch data task / processing task / shutdown task
	consumer.setTaskDoneFlag(false)
	switch consumer.getConsumerStatus() {
	case INITIALIZING:
		go func() {
			cursor, err := consumer.consumerInitializeTask()
			if err == nil {
				consumer.nextFetchCursor = cursor
			}
			consumer.updateStatus(err == nil)
		}()
	case PULLING:
		go func() {
			if !consumer.shouldFetch() {
				level.Debug(consumer.logger).Log("msg", "Pull Log Current Limitation and Re-Pull Log")
				consumer.updateStatus(false)
				return
			}
			err := consumer.nextFetchTask()
			consumer.updateStatus(err == nil && consumer.lastFetchGroupCount > 0)
		}()
	case PROCESSING:
		go func() {
			rollBackCheckpoint := consumer.consumerProcessTask()
			if rollBackCheckpoint != "" {
				consumer.nextFetchCursor = rollBackCheckpoint
				level.Info(consumer.logger).Log(
					"msg", "Checkpoints set for users have been reset",
					"shardId", consumer.shardId,
					"rollBackCheckpoint", rollBackCheckpoint,
				)
			}
			consumer.updateStatus(true)
		}()
	case SHUTTING_DOWN:
		go func() {
			err := consumer.consumerCheckPointTracker.flushCheckPoint()
			if err == nil {
				level.Info(consumer.logger).Log("msg", "shard worker status shutdown_complete", "shardWorkerId", consumer.shardId)
			} else {
				level.Warn(consumer.logger).Log("msg", "failed to flush checkpoint when shutdown", "err", err)
			}

			consumer.updateStatus(err == nil)
		}()
	default:
		consumer.setTaskDoneFlag(true)
	}
}

func (consumer *ShardConsumerWorker) updateStatus(success bool) {
	status := consumer.getConsumerStatus()
	if status == SHUTTING_DOWN {
		if success {
			consumer.setConsumerStatus(SHUTDOWN_COMPLETE)
		}
	} else if consumer.shutdownFlag {
		consumer.setConsumerStatus(SHUTTING_DOWN)
	} else if success {
		switch status {
		case INITIALIZING, PULLING:
			consumer.setConsumerStatus(PROCESSING)
		case PROCESSING:
			consumer.setConsumerStatus(PULLING)
		}
	}

	consumer.setTaskDoneFlag(true)
}

func (consumer *ShardConsumerWorker) shouldFetch() bool {
	if consumer.lastFetchGroupCount >= 1000 {
		return true
	}
	duration := time.Since(consumer.lastFetchTime)
	if consumer.lastFetchGroupCount < 100 {
		// The time used here is in milliseconds.
		return duration > 500*time.Millisecond
	} else if consumer.lastFetchGroupCount < 500 {
		return duration > 200*time.Millisecond
	} else { // 500 - 1000
		return duration > 50*time.Millisecond
	}
}

func (consumer *ShardConsumerWorker) saveCheckPointIfNeeded() {
	if consumer.client.option.AutoCommitDisabled {
		return
	}
	if time.Since(consumer.lastCheckpointSaveTime) > time.Millisecond*time.Duration(consumer.client.option.AutoCommitIntervalInMS) {
		consumer.consumerCheckPointTracker.flushCheckPoint()
		consumer.lastCheckpointSaveTime = time.Now()
	}
}

func (consumer *ShardConsumerWorker) consumerShutDown() {
	consumer.shutdownFlag = true
	if !consumer.isShutDownComplete() {
		consumer.consume()
	}
}

func (consumer *ShardConsumerWorker) isShutDownComplete() bool {
	return consumer.getConsumerStatus() == SHUTDOWN_COMPLETE
}

func (consumer *ShardConsumerWorker) setTaskDoneFlag(done bool) {
	consumer.taskLock.Lock()
	defer consumer.taskLock.Unlock()
	consumer.isCurrentDone = done
}

func (consumer *ShardConsumerWorker) isTaskDone() bool {
	consumer.taskLock.RLock()
	defer consumer.taskLock.RUnlock()
	return consumer.isCurrentDone
}
