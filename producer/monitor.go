package producer

import (
	"sync/atomic"
	"time"

	"github.com/aliyun/aliyun-log-go-sdk/internal"
	"github.com/go-kit/kit/log"
)

type ProducerMetrics struct {
	send       internal.TimeHistogram // send logs
	retryCount atomic.Int64

	onFail    internal.TimeHistogram // onSuccess callback
	onSuccess internal.TimeHistogram // onFail callback

	waitMemory          internal.TimeHistogram
	waitMemoryFailCount atomic.Int64
}

type ProducerMonitor struct {
	metrics atomic.Value // *ProducerMetrics
}

func newProducerMonitor() *ProducerMonitor {
	m := &ProducerMonitor{}
	m.metrics.Store(&ProducerMetrics{})
	return m
}

func (m *ProducerMonitor) recordSuccess(sendBegin time.Time, sendEnd time.Time) {
	metrics := m.metrics.Load().(*ProducerMetrics)
	metrics.send.AddSample(float64(sendEnd.Sub(sendBegin).Microseconds()))
	metrics.onSuccess.AddSample(float64(time.Since(sendEnd).Microseconds()))
}

func (m *ProducerMonitor) recordFailure(sendBegin time.Time, sendEnd time.Time) {
	metrics := m.metrics.Load().(*ProducerMetrics)
	metrics.send.AddSample(float64(sendEnd.Sub(sendBegin).Microseconds()))
	metrics.onFail.AddSample(float64(time.Since(sendEnd).Microseconds()))
}

func (m *ProducerMonitor) recordRetry(sendCost time.Duration) {
	metrics := m.metrics.Load().(*ProducerMetrics)
	metrics.send.AddSample(float64(sendCost.Microseconds()))
	metrics.retryCount.Add(1)
}

func (m *ProducerMonitor) recordWaitMemory(start time.Time) {
	metrics := m.metrics.Load().(*ProducerMetrics)
	metrics.waitMemory.AddSample(float64(time.Since(start).Microseconds()))
}

func (m *ProducerMonitor) incWaitMemoryFail() {
	metrics := m.metrics.Load().(*ProducerMetrics)
	metrics.waitMemoryFailCount.Add(1)
}

func (m *ProducerMonitor) getAndResetMetrics() *ProducerMetrics {
	// we dont need cmp and swap, only one thread would call m.metrics.Store
	old := m.metrics.Load().(*ProducerMetrics)
	m.metrics.Store(&ProducerMetrics{})
	return old
}

func (m *ProducerMonitor) reportThread(reportInterval time.Duration, logger log.Logger) {
	ticker := time.NewTicker(reportInterval)
	for range ticker.C {
		metrics := m.getAndResetMetrics()
		logger.Log("msg", "report status",
			"send", metrics.send.String(),
			"retryCount", metrics.retryCount.Load(),
			"onSuccess", metrics.onSuccess.String(),
			"onFail", metrics.onFail.String(),
			"waitMemory", metrics.waitMemory.String(),
			"waitMemoryFailCount", metrics.waitMemoryFailCount.Load(),
		)
	}
}