package consumerLibrary

import (
	"errors"
	"fmt"

	"github.com/go-kit/kit/log/level"
)

// return beginCursor, endCursor, error
func (consumer *ShardConsumerWorker) consumerInitializeTask() (string, string, error) {
	// read checkpoint firstly
	checkpoint, err := consumer.client.getCheckPoint(consumer.shardId)
	if err != nil {
		return "", "", err
	}
	if checkpoint != "" {
		consumer.consumerCheckPointTracker.initCheckPoint(checkpoint)
		return checkpoint, "", nil
	}

	if consumer.client.option.CursorPosition == BEGIN_CURSOR {
		cursor, err := consumer.client.getCursor(consumer.shardId, "begin")
		if err != nil {
			level.Warn(consumer.logger).Log("msg", "get beginCursor error", "error", err)
		}
		return cursor, "", err
	}
	if consumer.client.option.CursorPosition == END_CURSOR {
		cursor, err := consumer.client.getCursor(consumer.shardId, "end")
		if err != nil {
			level.Warn(consumer.logger).Log("msg", "get endCursor error", "error", err)
		}
		return cursor, "", err
	}
	if consumer.client.option.CursorPosition == SPECIAL_TIMER_CURSOR {
		beginCursor, endCursor, err := consumer.getCursorByTime()
		if err != nil {
			return "", "", err
		}
		return beginCursor, endCursor, nil
	}
	level.Warn(consumer.logger).Log("msg", "CursorPosition setting error, please reset with BEGIN_CURSOR or END_CURSOR or SPECIAL_TIMER_CURSOR")
	return "", "", errors.New("CursorPositionError")
}

func (consumer *ShardConsumerWorker) getCursorByTime() (beginCursor string, endCursor string, err error) {
	beginCursor, err = consumer.client.getCursor(consumer.shardId, fmt.Sprintf("%v", consumer.client.option.CursorStartTime))
	if err != nil {
		level.Warn(consumer.logger).Log("msg", "get specialCursor error", "error", err)
		return "", "", err
	}

	if consumer.client.option.CursorEndTime == 0 {
		return beginCursor, "", nil
	}
	endCursor, err = consumer.client.getCursor(consumer.shardId, fmt.Sprintf("%v", consumer.client.option.CursorEndTime))
	if err != nil {
		level.Warn(consumer.logger).Log("msg", "get specialCursor for endCursor error", "error", err)
		return "", "", err
	}
	return beginCursor, endCursor, nil
}
