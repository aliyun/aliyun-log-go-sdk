package producer

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/go-kit/kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

const tagsTestURL = "http://tags-project.cn-mock.example.com/logstores/tags-store"

func newTagsTestProducer(t *testing.T, configure func(*ProducerConfig)) (*Producer, *httpmock.MockTransport) {
	t.Helper()
	transport := testutil.NewMockTransport() // Unmatched requests fail; never fall back to the network.
	config := GetDefaultProducerConfig()
	config.Endpoint = "cn-mock.example.com"
	config.CredentialsProvider = sls.NewStaticCredentialsProvider("mock-id", "mock-key", "")
	config.HTTPClient = &http.Client{Transport: transport, Timeout: time.Second}
	config.Logger = log.NewNopLogger()
	config.DisableRuntimeMetrics = true
	config.CompressType = sls.Compress_None
	config.AdjustShargHash = false
	config.MaxBlockSec = 0
	config.MaxBatchCount = 1024
	config.MaxBatchSize = 1024 * 1024
	config.MaxIoWorkerCount = 2
	config.LingerMs = 100
	if configure != nil {
		configure(config)
	}
	p, err := NewProducer(config)
	require.NoError(t, err)
	// Initialize timeouts before concurrent requests to avoid lazy initialization races.
	client := p.mover.ioWorker.client.(*sls.Client)
	client.RequestTimeOut, client.RetryTimeOut = time.Second, time.Second
	return p, transport
}

func tagsTestTag(key, value string) *sls.LogTag {
	return &sls.LogTag{Key: proto.String(key), Value: proto.String(value)}
}

func tagsTestLogs(ids ...string) []*sls.Log {
	logs := make([]*sls.Log, 0, len(ids))
	for _, id := range ids {
		logs = append(logs, &sls.Log{
			Time:     proto.Uint32(1700000000),
			Contents: []*sls.LogContent{{Key: proto.String("id"), Value: proto.String(id)}},
		})
	}
	return logs
}

func tagsTestIDs(logs []*sls.Log) []string {
	ids := make([]string, 0, len(logs))
	for _, entry := range logs {
		ids = append(ids, entry.GetContents()[0].GetValue())
	}
	return ids
}

// Pairs preserve order and multiplicity, unlike a map keyed only by tag key.
func tagsTestPairs(tags []*sls.LogTag) [][2]string {
	pairs := make([][2]string, 0, len(tags))
	for _, tag := range tags {
		pairs = append(pairs, [2]string{tag.GetKey(), tag.GetValue()})
	}
	return pairs
}

// Inspect only unstarted producers, or producers whose senders have finished.
func tagsTestBatches(p *Producer) []*ProducerBatch {
	p.logAccumulator.lock.Lock()
	defer p.logAccumulator.lock.Unlock()
	var batches []*ProducerBatch
	for _, batch := range p.logAccumulator.logGroupData {
		if batch != nil {
			batches = append(batches, batch)
		}
	}
	return batches
}

func tagsTestOnlyBatch(t *testing.T, p *Producer) *ProducerBatch {
	t.Helper()
	batches := tagsTestBatches(p)
	require.Len(t, batches, 1)
	return batches[0]
}

func tagsTestAwait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for producer")
		var zero T
		return zero
	}
}

type tagsTestCallbackEvent struct {
	success bool
	result  *Result
}

type tagsTestCallback struct{ events chan tagsTestCallbackEvent }

func (cb *tagsTestCallback) Success(result *Result) {
	cb.events <- tagsTestCallbackEvent{success: true, result: result}
}

func (cb *tagsTestCallback) Fail(result *Result) {
	cb.events <- tagsTestCallbackEvent{result: result}
}

type tagsTestRequest struct {
	group     *sls.LogGroup
	body      []byte
	url       string
	arrivedMs int64
	err       error
}

// HTTP 400 exercises producer retries without triggering SDK client retries.
func tagsTestCapture(transport *httpmock.MockTransport, statuses ...int) <-chan tagsTestRequest {
	requests := make(chan tagsTestRequest, 32)
	var calls int64
	transport.RegisterResponder(http.MethodPost,
		`=~^http://tags-project\.cn-mock\.example\.com/logstores/tags-store(?:/shards/route\?key=.*)?$`,
		func(req *http.Request) (*http.Response, error) {
			observation := tagsTestRequest{group: &sls.LogGroup{}, url: req.URL.String(), arrivedMs: time.Now().UnixMilli()}
			observation.body, observation.err = io.ReadAll(req.Body)
			if observation.err == nil {
				observation.err = proto.Unmarshal(observation.body, observation.group)
			}
			requests <- observation
			index := int(atomic.AddInt64(&calls, 1)) - 1
			if index >= len(statuses) {
				index = len(statuses) - 1
			}
			status := statuses[index]
			body := ""
			if status != http.StatusOK {
				body = `{"errorCode":"MockRejected","errorMessage":"mock rejection"}`
			}
			return &http.Response{
				StatusCode: status,
				Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
				Header:     http.Header{"X-Log-Requestid": []string{"tags-request"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		})
	return requests
}

func tagsTestStart(t *testing.T, p *Producer) func() {
	t.Helper()
	p.Start()
	var once sync.Once
	closeProducer := func() {
		once.Do(func() {
			done := make(chan struct{})
			go func() { p.SafeClose(); close(done) }()
			tagsTestAwait(t, done)
		})
	}
	t.Cleanup(closeProducer)
	return closeProducer
}

func TestProducerTagsAggregation(t *testing.T) {
	p, _ := newTagsTestProducer(t, nil)
	first := []*sls.LogTag{tagsTestTag("z", "9"), tagsTestTag("a", "1"), tagsTestTag("a", "1")}
	require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("1", "2"), first))
	batch := tagsTestOnlyBatch(t, p)
	reordered := []*sls.LogTag{tagsTestTag("a", "1"), tagsTestTag("z", "9"), tagsTestTag("a", "1")}
	require.NoError(t, p.SendLogListWithTagsAndCallBack("p", "s", "t", "src", tagsTestLogs("3", "4"), reordered, nil))
	require.Same(t, batch, tagsTestOnlyBatch(t, p))
	require.Equal(t, []string{"1", "2", "3", "4"}, tagsTestIDs(batch.logGroup.Logs))
	require.Equal(t, tagsTestPairs(first), tagsTestPairs(batch.logGroup.LogTags), "keep the first call's order")

	// Keys, values, and duplicate counts all participate in identity.
	variants := [][]*sls.LogTag{
		{tagsTestTag("z", "9"), tagsTestTag("b", "1"), tagsTestTag("a", "1")},
		{tagsTestTag("z", "9"), tagsTestTag("a", "2"), tagsTestTag("a", "1")},
		{tagsTestTag("z", "9"), tagsTestTag("a", "1")},
		{tagsTestTag("z", "9"), tagsTestTag("a", "1"), tagsTestTag("a", "1"), tagsTestTag("a", "1")},
	}
	for i, tags := range variants {
		require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs(fmt.Sprint(i)), tags))
		require.Len(t, tagsTestBatches(p), i+2)
	}

	require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("nil"), nil))
	require.NoError(t, p.SendLogListWithTagsAndCallBack("p", "s", "t", "src", tagsTestLogs("empty"), []*sls.LogTag{}, nil))
	require.NoError(t, p.SendLogList("p", "s", "t", "src", tagsTestLogs("legacy")))
	require.Len(t, tagsTestBatches(p), 6)
	for _, b := range tagsTestBatches(p) {
		if len(b.logGroup.LogTags) == 0 {
			require.Equal(t, []string{"nil", "empty", "legacy"}, tagsTestIDs(b.logGroup.Logs))
		}
	}
	require.Empty(t, p.threadPool.taskCh)
}

func TestProducerTagsPathIsolation(t *testing.T) {
	p, _ := newTagsTestProducer(t, nil)
	paths := [][4]string{
		{"p", "s", "t", "src"},
		{"other-p", "s", "t", "src"},
		{"p", "other-s", "t", "src"},
		{"p", "s", "other-t", "src"},
		{"p", "s", "t", "other-src"},
	}
	for i, path := range paths {
		for call := 0; call < 2; call++ {
			require.NoError(t, p.SendLogListWithTags(path[0], path[1], path[2], path[3], tagsTestLogs(strconv.Itoa(i)), []*sls.LogTag{tagsTestTag("k", "v")}))
		}
	}
	batches := tagsTestBatches(p)
	require.Len(t, batches, len(paths))
	for _, batch := range batches {
		ids := tagsTestIDs(batch.logGroup.Logs)
		require.Len(t, ids, 2)
		require.Equal(t, ids[0], ids[1])
		i, err := strconv.Atoi(ids[0])
		require.NoError(t, err)
		require.Equal(t, paths[i], [4]string{batch.project, batch.logstore, batch.logGroup.GetTopic(), batch.logGroup.GetSource()})
		require.Nil(t, batch.getShardHash())
	}
}

func TestProducerTagsNoKeyCollisions(t *testing.T) {
	p, _ := newTagsTestProducer(t, nil)
	type input struct {
		path [5]string // project, logstore, topic, source, shardHash
		tags []*sls.LogTag
	}
	var inputs []input
	add := func(path [5]string, tags []*sls.LogTag) {
		id := strconv.Itoa(len(inputs))
		inputs = append(inputs, input{path, tags})
		var err error
		if path[4] != "" {
			err = p.HashSendLogList(path[0], path[1], path[4], path[2], path[3], tagsTestLogs(id))
		} else {
			err = p.SendLogListWithTags(path[0], path[1], path[2], path[3], tagsTestLogs(id), tags)
		}
		require.NoError(t, err)
	}
	base := [5]string{"p", "s", "t", "src", ""}
	// This cross-product includes empty strings, likely delimiters, UTF-8 and NUL.
	words := []string{"", "$", "|", ":", "=", "a$b", "a|b", "a:b", "中文", "\x00", "键\x00值"}
	for _, key := range words {
		for _, value := range words {
			add(base, []*sls.LogTag{tagsTestTag(key, value)})
		}
	}
	add(base, nil) // Distinct from one tag whose key and value are both empty.
	add(base, []*sls.LogTag{tagsTestTag("a", "b|c=d")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b$c=d")})
	add(base, []*sls.LogTag{tagsTestTag("a$b", "c")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b$c")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b"), tagsTestTag("c", "d")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b"), tagsTestTag("c", "d"), tagsTestTag("c", "d")})
	add(base, []*sls.LogTag{tagsTestTag("a|b", "c")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b|c")})
	add(base, []*sls.LogTag{tagsTestTag("a\x00b", "c")})
	add(base, []*sls.LogTag{tagsTestTag("a", "b\x00c")})
	for _, path := range [][5]string{
		{"p-s", "x", "t", "src", ""}, {"p", "s-x", "t", "src", ""},
		{"p", "s_t", "x", "src", ""}, {"p", "s", "t|x", "src", ""},
		{"p", "s", "t||x", "src", ""}, {"p", "s", "t", "x||src", ""},
		{"p", "s", "t|h", "src", "x"}, {"p", "s", "t", "src", "h|x"},
		{"p", "s", "t", "src", "h|x|y"}, {"p", "s", "t", "y|src", "h|x"},
		{"p", "s", "", "", ""}, {"p", "s", "\x00", "来源", ""},
	} {
		add(path, nil)
	}
	// A source suffix must not be mistaken for dynamic tags.
	add([5]string{"p", "s", "t", "src|k|v", ""}, nil)
	add(base, []*sls.LogTag{tagsTestTag("k", "v")})

	batches := tagsTestBatches(p)
	require.Len(t, batches, len(inputs))
	for _, batch := range batches {
		require.Len(t, batch.logGroup.Logs, 1, "distinct inputs must never share a batch")
		index, err := strconv.Atoi(tagsTestIDs(batch.logGroup.Logs)[0])
		require.NoError(t, err)
		want := inputs[index]
		hash := ""
		if batch.getShardHash() != nil {
			hash = *batch.getShardHash()
		}
		require.Equal(t, want.path, [5]string{batch.project, batch.logstore, batch.logGroup.GetTopic(), batch.logGroup.GetSource(), hash})
		require.Equal(t, tagsTestPairs(want.tags), tagsTestPairs(batch.logGroup.LogTags))
	}
}

func TestProducerTagsInjectionAndSnapshot(t *testing.T) {
	for _, packID := range []bool{false, true} {
		t.Run(fmt.Sprintf("packID=%t", packID), func(t *testing.T) {
			dynamicBacking := []*sls.LogTag{
				tagsTestTag("z", "first"), tagsTestTag("__pack_id__", "dynamic"), tagsTestTag("same", "value"),
				tagsTestTag("spare", "1"), tagsTestTag("spare", "2"), tagsTestTag("spare", "3"),
			}
			fixedBacking := []*sls.LogTag{
				tagsTestTag("same", "value"), tagsTestTag("__pack_id__", "fixed"), tagsTestTag("fixed-spare", "untouched"),
			}
			dynamic := dynamicBacking[:3]
			fixed := fixedBacking[:2]
			dynamicPointers := append([]*sls.LogTag(nil), dynamicBacking...)
			fixedPointers := append([]*sls.LogTag(nil), fixedBacking...)
			dynamicBefore, fixedBefore := tagsTestPairs(dynamicBacking), tagsTestPairs(fixedBacking)
			p, _ := newTagsTestProducer(t, func(c *ProducerConfig) { c.GeneratePackId, c.LogTags = packID, fixed })
			require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("first"), dynamic))
			batch := tagsTestOnlyBatch(t, p)
			want := append(tagsTestPairs(dynamic), tagsTestPairs(fixed)...)
			if packID {
				require.Len(t, batch.logGroup.LogTags, len(want)+1)
				generated := batch.logGroup.LogTags[len(want)]
				require.Equal(t, "__pack_id__", generated.GetKey())
				require.NotEmpty(t, generated.GetValue())
				want = append(want, [2]string{"__pack_id__", generated.GetValue()})
			}
			require.Equal(t, want, tagsTestPairs(batch.logGroup.LogTags))
			require.Equal(t, dynamicBefore, tagsTestPairs(dynamicBacking), "do not sort or append into caller capacity")
			require.Equal(t, fixedBefore, tagsTestPairs(fixedBacking), "do not append packId into config capacity")
			for i, tag := range dynamicPointers {
				require.Same(t, tag, dynamicBacking[i])
			}
			for i, tag := range fixedPointers {
				require.Same(t, tag, fixedBacking[i])
			}
			for i, tag := range dynamic {
				require.NotSame(t, tag, batch.logGroup.LogTags[i])
				require.NotSame(t, tag.Key, batch.logGroup.LogTags[i].Key)
				require.NotSame(t, tag.Value, batch.logGroup.LogTags[i].Value)
			}

			*dynamic[0].Key, *dynamic[0].Value = "mutated-key", "mutated-value"
			dynamic[1].Key, dynamic[1].Value = proto.String("replaced-key"), proto.String("replaced-value")
			dynamic[2] = tagsTestTag("replacement", "object")
			require.Equal(t, want, tagsTestPairs(batch.logGroup.LogTags))
			// Config tags are injected only at creation, not included in aggregation identity.
			p.producerConfig.LogTags = []*sls.LogTag{tagsTestTag("new-fixed", "next-batch")}
			reordered := []*sls.LogTag{tagsTestTag("same", "value"), tagsTestTag("__pack_id__", "dynamic"), tagsTestTag("z", "first")}
			require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("same"), reordered))
			require.Same(t, batch, tagsTestOnlyBatch(t, p))
			require.Equal(t, want, tagsTestPairs(batch.logGroup.LogTags))
			require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("other"), []*sls.LogTag{tagsTestTag("other", "tag")}))
			require.Len(t, tagsTestBatches(p), 2)
			for _, other := range tagsTestBatches(p) {
				if other == batch {
					continue
				}
				otherWant := [][2]string{{"other", "tag"}, {"new-fixed", "next-batch"}}
				if packID {
					require.Len(t, other.logGroup.LogTags, 3)
					otherPack := other.logGroup.LogTags[2].GetValue()
					require.NotEqual(t, want[len(want)-1][1], otherPack)
					otherWant = append(otherWant, [2]string{"__pack_id__", otherPack})
				}
				require.Equal(t, otherWant, tagsTestPairs(other.logGroup.LogTags))
			}
		})
	}
}

func TestProducerTagsCallbacksAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []int
		success  bool
	}{
		{"success", []int{200}, true},
		{"retry-success", []int{400, 400, 200}, true},
		{"retry-exhausted", []int{400, 400, 400}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, transport := newTagsTestProducer(t, func(c *ProducerConfig) {
				c.MaxBatchCount, c.Retries = 4, 2
				c.GeneratePackId = true
				c.LogTags = []*sls.LogTag{tagsTestTag("fixed", "value")}
				c.NoRetryStatusCodeList = nil // Let the producer, but not the SDK client, retry HTTP 400.
			})
			requests := tagsTestCapture(transport, tc.statuses...)
			callbacks := []*tagsTestCallback{{make(chan tagsTestCallbackEvent, 4)}, {make(chan tagsTestCallbackEvent, 4)}}
			dynamic := []*sls.LogTag{tagsTestTag("z", "9"), tagsTestTag("a", "1")}
			require.NoError(t, p.SendLogListWithTagsAndCallBack("tags-project", "tags-store", "topic", "source", tagsTestLogs("1", "2"), dynamic, callbacks[0]))
			require.NoError(t, p.SendLogListWithTagsAndCallBack("tags-project", "tags-store", "topic", "source", tagsTestLogs("3", "4"), []*sls.LogTag{tagsTestTag("a", "1"), tagsTestTag("z", "9")}, callbacks[1]))
			batch := tagsTestAwait(t, p.threadPool.taskCh)
			require.Empty(t, tagsTestBatches(p))
			*dynamic[0].Value = "changed after enqueue"
			var firstBody []byte
			for i := range tc.statuses {
				p.mover.ioWorker.sendToServer(batch)
				request := tagsTestAwait(t, requests)
				require.NoError(t, request.err, "wire body must be uncompressed protobuf")
				require.Equal(t, tagsTestURL, request.url)
				require.Equal(t, "topic", request.group.GetTopic())
				require.Equal(t, "source", request.group.GetSource())
				require.Equal(t, []string{"1", "2", "3", "4"}, tagsTestIDs(request.group.Logs))
				require.Len(t, request.group.LogTags, 4)
				require.Equal(t, [][2]string{{"z", "9"}, {"a", "1"}, {"fixed", "value"}}, tagsTestPairs(request.group.LogTags[:3]))
				require.Equal(t, "__pack_id__", request.group.LogTags[3].GetKey())
				require.NotEmpty(t, request.group.LogTags[3].GetValue())
				if i == 0 {
					firstBody = request.body
				} else {
					require.Equal(t, firstBody, request.body, "retries must preserve tags and packId")
				}
				if i < len(tc.statuses)-1 {
					for _, cb := range callbacks {
						require.Empty(t, cb.events, "no callback on an intermediate failure")
					}
					require.Equal(t, batch.totalDataSize, atomic.LoadInt64(&p.producerLogGroupSize))
					queued := p.mover.retryQueue.getRetryBatch(true) // Drain directly, without waiting for backoff.
					require.Len(t, queued, 1)
					require.Same(t, batch, queued[0])
				}
			}
			for _, cb := range callbacks {
				event := tagsTestAwait(t, cb.events)
				require.Equal(t, tc.success, event.success)
				require.Equal(t, tc.success, event.result.IsSuccessful())
				attempts := event.result.GetReservedAttempts()
				require.Len(t, attempts, len(tc.statuses))
				for i, attempt := range attempts {
					require.Equal(t, tc.statuses[i] == 200, attempt.Success)
				}
				if !tc.success {
					require.Equal(t, "MockRejected", event.result.GetErrorCode())
					require.Equal(t, "mock rejection", event.result.GetErrorMessage())
					require.Equal(t, "tags-request", event.result.GetRequestId())
				}
				require.Empty(t, cb.events, "exactly one terminal callback per call")
			}
			require.Zero(t, atomic.LoadInt64(&p.producerLogGroupSize))
			require.Empty(t, p.mover.retryQueue.getRetryBatch(true))
			require.Equal(t, len(tc.statuses), transport.GetTotalCallCount())
		})
	}
}

func TestProducerTagsLegacyAPIsAndHash(t *testing.T) {
	for _, adjust := range []bool{false, true} {
		t.Run(fmt.Sprintf("adjust=%t", adjust), func(t *testing.T) {
			p, transport := newTagsTestProducer(t, func(c *ProducerConfig) { c.AdjustShargHash = adjust })
			requests := tagsTestCapture(transport, 200)
			const project, store = "tags-project", "tags-store"
			expected := map[string][]string{"": {"single", "list", "single-cb", "list-cb", "new-nil", "new-empty"}}
			require.NoError(t, p.SendLog(project, store, "t", "src", tagsTestLogs("single")[0]))
			require.NoError(t, p.SendLogList(project, store, "t", "src", tagsTestLogs("list")))
			require.NoError(t, p.SendLogWithCallBack(project, store, "t", "src", tagsTestLogs("single-cb")[0], nil))
			require.NoError(t, p.SendLogListWithCallBack(project, store, "t", "src", tagsTestLogs("list-cb"), nil))
			require.NoError(t, p.SendLogListWithTags(project, store, "t", "src", tagsTestLogs("new-nil"), nil))
			require.NoError(t, p.SendLogListWithTagsAndCallBack(project, store, "t", "src", tagsTestLogs("new-empty"), []*sls.LogTag{}, nil))
			for _, hash := range []string{"", "hash-a", "hash-b"} {
				ids := []string{hash + "/single", hash + "/list", hash + "/single-cb", hash + "/list-cb"}
				require.NoError(t, p.HashSendLog(project, store, hash, "t", "src", tagsTestLogs(ids[0])[0]))
				require.NoError(t, p.HashSendLogList(project, store, hash, "t", "src", tagsTestLogs(ids[1])))
				require.NoError(t, p.HashSendLogWithCallBack(project, store, hash, "t", "src", tagsTestLogs(ids[2])[0], nil))
				require.NoError(t, p.HashSendLogListWithCallBack(project, store, hash, "t", "src", tagsTestLogs(ids[3]), nil))
				wantHash := hash
				if adjust {
					var err error
					wantHash, err = AdjustHash(hash, p.producerConfig.Buckets)
					require.NoError(t, err)
					require.NotEmpty(t, wantHash, "legacy hash APIs adjust even an empty hash")
				}
				expected[wantHash] = append(expected[wantHash], ids...)
			}
			require.Len(t, tagsTestBatches(p), len(expected))
			for _, batch := range tagsTestBatches(p) {
				hash, wantURL := "", tagsTestURL
				if batch.getShardHash() != nil {
					hash = *batch.getShardHash()
					require.NotEmpty(t, hash, "empty shardHash is represented by nil")
					wantURL += "/shards/route?key=" + hash
				}
				require.Equal(t, expected[hash], tagsTestIDs(batch.logGroup.Logs))
				require.Empty(t, batch.logGroup.LogTags)
				p.mover.ioWorker.sendToServer(batch)
				request := tagsTestAwait(t, requests)
				require.NoError(t, request.err)
				require.Equal(t, wantURL, request.url)
				require.Equal(t, expected[hash], tagsTestIDs(request.group.Logs))
				delete(expected, hash)
			}
			require.Empty(t, expected)
			require.Zero(t, atomic.LoadInt64(&p.producerLogGroupSize))
		})
	}
}

func TestProducerTagsThresholds(t *testing.T) {
	for _, threshold := range []string{"count", "size"} {
		for _, crossing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/crossing=%t", threshold, crossing), func(t *testing.T) {
				p, _ := newTagsTestProducer(t, func(c *ProducerConfig) {
					if threshold == "count" {
						c.MaxBatchCount = 3
					} else {
						c.MaxBatchSize = int64(3 * GetLogListSize(tagsTestLogs("x")))
					}
				})
				tags := []*sls.LogTag{tagsTestTag("z", "9"), tagsTestTag("a", "1")}
				require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("x", "x"), tags))
				original := tagsTestOnlyBatch(t, p)
				require.Empty(t, p.threadPool.taskCh)
				// An unrelated tag set must not contribute to this batch's threshold.
				require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("x"), []*sls.LogTag{tagsTestTag("other", "tag")}))
				require.Empty(t, p.threadPool.taskCh)
				last := tagsTestLogs("x")
				if crossing {
					last = tagsTestLogs("x", "x")
				}
				reversed := []*sls.LogTag{tagsTestTag("a", "1"), tagsTestTag("z", "9")}
				require.NoError(t, p.SendLogListWithTagsAndCallBack("p", "s", "t", "src", last, reversed, nil))
				sealed := tagsTestAwait(t, p.threadPool.taskCh)
				require.Same(t, original, sealed)
				require.Len(t, sealed.logGroup.Logs, 2+len(last))
				require.Equal(t, tagsTestPairs(tags), tagsTestPairs(sealed.logGroup.LogTags))
				require.Empty(t, p.threadPool.taskCh)
				require.Len(t, tagsTestBatches(p), 1)
				require.NoError(t, p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs("x"), reversed))
				require.Len(t, tagsTestBatches(p), 2)
				for _, batch := range tagsTestBatches(p) {
					require.NotSame(t, sealed, batch)
					if len(batch.logGroup.LogTags) == 2 {
						require.Equal(t, tagsTestPairs(reversed), tagsTestPairs(batch.logGroup.LogTags), "new batch uses its own first call's order")
					}
				}
			})
		}
	}
}

func TestProducerTagsLingerAndSafeClose(t *testing.T) {
	for _, closeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("safeClose=%t", closeFirst), func(t *testing.T) {
			p, transport := newTagsTestProducer(t, nil)
			requests := tagsTestCapture(transport, 200)
			cb := &tagsTestCallback{make(chan tagsTestCallbackEvent, 8)}
			require.NoError(t, p.SendLogListWithTagsAndCallBack("tags-project", "tags-store", "topic", "source", tagsTestLogs("red-1", "red-2"), []*sls.LogTag{tagsTestTag("route", "red"), tagsTestTag("k", "v")}, cb))
			require.NoError(t, p.SendLogListWithTagsAndCallBack("tags-project", "tags-store", "topic", "source", tagsTestLogs("red-3"), []*sls.LogTag{tagsTestTag("k", "v"), tagsTestTag("route", "red")}, cb))
			require.NoError(t, p.SendLogListWithTagsAndCallBack("tags-project", "tags-store", "topic", "source", tagsTestLogs("blue"), []*sls.LogTag{tagsTestTag("route", "blue")}, cb))
			require.Len(t, tagsTestBatches(p), 2)
			require.Empty(t, p.threadPool.taskCh)
			created := time.Now().UnixMilli()
			for _, batch := range tagsTestBatches(p) {
				batch.createTimeMs = created
				if closeFirst {
					// Keep batches unexpired regardless of scheduling: only shutdown can flush them.
					batch.createTimeMs += time.Hour.Milliseconds()
				}
			}
			closeProducer := tagsTestStart(t, p)
			if closeFirst {
				closeProducer()
			}
			for i := 0; i < 3; i++ {
				event := tagsTestAwait(t, cb.events)
				require.True(t, event.success)
				require.True(t, event.result.IsSuccessful())
			}
			closeProducer()
			seen := make(map[string][]string)
			for i := 0; i < 2; i++ {
				request := tagsTestAwait(t, requests)
				require.NoError(t, request.err)
				require.Equal(t, tagsTestURL, request.url)
				if !closeFirst {
					require.GreaterOrEqual(t, request.arrivedMs, created+p.producerConfig.LingerMs)
				}
				route := request.group.LogTags[0].GetValue()
				seen[route] = tagsTestIDs(request.group.Logs)
			}
			require.Equal(t, map[string][]string{"red": {"red-1", "red-2", "red-3"}, "blue": {"blue"}}, seen)
			require.Empty(t, cb.events)
			require.Empty(t, requests)
			require.Empty(t, tagsTestBatches(p))
			require.Empty(t, p.threadPool.taskCh)
			require.Zero(t, atomic.LoadInt64(&p.producerLogGroupSize))
			require.True(t, p.threadPool.Stopped())
		})
	}
}

func TestProducerTagsRejectedInputsDoNotEnqueue(t *testing.T) {
	for _, callbackAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("callback=%t", callbackAPI), func(t *testing.T) {
			cb := &tagsTestCallback{make(chan tagsTestCallbackEvent, 8)}
			send := func(p *Producer, tags []*sls.LogTag, logs ...*sls.Log) error {
				if callbackAPI {
					return p.SendLogListWithTagsAndCallBack("p", "s", "t", "src", logs, tags, cb)
				}
				return p.SendLogListWithTags("p", "s", "t", "src", logs, tags)
			}
			for _, invalid := range []struct {
				name string
				tag  *sls.LogTag
			}{
				{"nil-tag", nil},
				{"nil-key", &sls.LogTag{Value: proto.String("v")}},
				{"nil-value", &sls.LogTag{Key: proto.String("k")}},
			} {
				t.Run(invalid.name, func(t *testing.T) {
					p, _ := newTagsTestProducer(t, nil)
					tags := []*sls.LogTag{tagsTestTag("valid", "prefix"), invalid.tag}
					for _, logs := range [][]*sls.Log{tagsTestLogs("x"), nil, {}} {
						require.Error(t, send(p, tags, logs...))
						require.Empty(t, p.logAccumulator.logGroupData, "reject before creating even an empty batch")
						require.Empty(t, p.threadPool.taskCh)
						require.Zero(t, atomic.LoadInt64(&p.producerLogGroupSize))
					}
				})
			}
			t.Run("closed", func(t *testing.T) {
				p, _ := newTagsTestProducer(t, nil)
				p.SafeClose() // No Start: no outstanding workers or data to drain.
				require.Error(t, send(p, []*sls.LogTag{tagsTestTag("k", "v")}, tagsTestLogs("x")...))
				require.Empty(t, p.logAccumulator.logGroupData)
				require.Empty(t, p.threadPool.taskCh)
				require.Zero(t, atomic.LoadInt64(&p.producerLogGroupSize))
			})
			for _, maxBlockSec := range []int{0, 1} {
				t.Run(fmt.Sprintf("backpressure/block=%d", maxBlockSec), func(t *testing.T) {
					p, _ := newTagsTestProducer(t, func(c *ProducerConfig) { c.TotalSizeLnBytes, c.MaxBlockSec = 1, maxBlockSec })
					require.NoError(t, send(p, []*sls.LogTag{tagsTestTag("k", "v")}, tagsTestLogs("x")...))
					before := atomic.LoadInt64(&p.producerLogGroupSize)
					batch := tagsTestOnlyBatch(t, p)
					errCh := make(chan error, 1)
					go func() { errCh <- send(p, []*sls.LogTag{tagsTestTag("other", "tag")}, tagsTestLogs("x")...) }()
					require.EqualError(t, tagsTestAwait(t, errCh), TimeoutExecption)
					require.Same(t, batch, tagsTestOnlyBatch(t, p))
					require.Len(t, batch.logGroup.Logs, 1)
					require.Equal(t, before, atomic.LoadInt64(&p.producerLogGroupSize))
					require.Empty(t, p.threadPool.taskCh)
				})
			}
			require.Empty(t, cb.events, "rejection is synchronous, not a terminal batch callback")
		})
	}
	// Empty lists and empty strings retain the old API's lack of business validation.
	for _, logs := range [][]*sls.Log{nil, {}} {
		p, _ := newTagsTestProducer(t, nil)
		tags := []*sls.LogTag{tagsTestTag("", "")}
		require.NoError(t, p.SendLogListWithTags("", "", "", "", logs, tags))
		require.NoError(t, p.SendLogListWithTagsAndCallBack("", "", "", "", logs, tags, nil))
	}
}

func TestProducerTagsConcurrentIsolation(t *testing.T) {
	p, _ := newTagsTestProducer(t, nil)
	const workers, calls, groups = 6, 12, 3
	start := make(chan struct{})
	done := make(chan struct{})
	errors := make(chan error, workers*calls)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for call := 0; call < calls; call++ {
				group := strconv.Itoa(worker % groups)
				tags := []*sls.LogTag{tagsTestTag("group", group), tagsTestTag("duplicate", "v"), tagsTestTag("duplicate", "v")}
				if call%2 == 1 {
					tags[0], tags[2] = tags[2], tags[0]
				}
				id := fmt.Sprintf("%s/%d/%d", group, worker, call)
				err := p.SendLogListWithTags("p", "s", "t", "src", tagsTestLogs(id+"/a", id+"/b"), tags)
				errors <- err
				// Mutation immediately after return also exercises snapshot safety under -race.
				for _, tag := range tags {
					*tag.Key, *tag.Value = "caller-mutated", "caller-mutated"
				}
			}
		}(worker)
	}
	close(start)
	go func() { wg.Wait(); close(done) }()
	tagsTestAwait(t, done)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	batches := tagsTestBatches(p)
	require.Len(t, batches, groups)
	seen := make(map[string]bool)
	for _, batch := range batches {
		require.Len(t, batch.logGroup.Logs, workers/groups*calls*2)
		ids := tagsTestIDs(batch.logGroup.Logs)
		group := strings.Split(ids[0], "/")[0]
		require.ElementsMatch(t, [][2]string{{"group", group}, {"duplicate", "v"}, {"duplicate", "v"}}, tagsTestPairs(batch.logGroup.LogTags))
		for _, id := range ids {
			require.True(t, strings.HasPrefix(id, group+"/"), "cross-group contamination: %s", id)
			require.False(t, seen[id], "duplicate log: %s", id)
			seen[id] = true
		}
	}
	require.Len(t, seen, workers*calls*2)
	require.Empty(t, p.threadPool.taskCh)
}

func TestProducerGetKeyStringEncoding(t *testing.T) {
	accumulator := &LogAccumulator{}
	for _, tags := range [][]*sls.LogTag{nil, {}} {
		require.Equal(t, "project$store$topic$$source", accumulator.getKeyString("project", "store", "topic", "", "source", tags))
		require.Equal(t, "project$store$topic$hash$source", accumulator.getKeyString("project", "store", "topic", "hash", "source", tags))
	}
	for _, size := range []int{0, 1, 255, 256, 4096, 65535, 65536} {
		value := strings.Repeat("v", size)
		var encoded strings.Builder
		encoded.Grow(4 + len(value))
		writeKeyField(&encoded, value)
		require.Equal(t, 4+len(value), encoded.Len())
		require.Equal(t, uint32(size), binary.LittleEndian.Uint32([]byte(encoded.String()[:4])))
		require.Equal(t, value, encoded.String()[4:])
		tags := []*sls.LogTag{tagsTestTag("k", value)}
		require.Equal(t, "project$store$topic$$source$\x01\x00\x00\x00k"+encoded.String(), accumulator.getKeyString("project", "store", "topic", "", "source", tags))
	}
}

func TestProducerGetKeyStringTagOrdering(t *testing.T) {
	accumulator := &LogAccumulator{}
	pairs := [][2]string{{"z", "last"}, {"a", "2"}, {"a", "1"}, {"a", "1"}, {"", ""}, {"a", ""}, {"a", "\x00$"}, {"中文", "值"}, {"a\x00", "v"}, {"aa", "v"}, {strings.Repeat("k", 256), strings.Repeat("v", 255)}}
	for _, count := range []int{0, 1, 2, 5, 15, 16, 17, 32, 128} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			backing := make([]*sls.LogTag, count+2)
			for i := range backing {
				pair := pairs[i%len(pairs)]
				backing[i] = tagsTestTag(pair[0], pair[1])
			}
			tags := backing[:count]
			wantPairs := tagsTestPairs(tags)
			sort.Slice(wantPairs, func(i, j int) bool {
				if wantPairs[i][0] != wantPairs[j][0] {
					return wantPairs[i][0] < wantPairs[j][0]
				}
				return wantPairs[i][1] < wantPairs[j][1]
			})
			var want strings.Builder
			want.WriteString("project$store$topic$$source")
			if count > 0 {
				want.WriteByte('$')
			}
			for _, pair := range wantPairs {
				for _, field := range pair {
					var length [4]byte
					binary.LittleEndian.PutUint32(length[:], uint32(len(field)))
					want.Write(length[:])
					want.WriteString(field)
				}
			}
			random := rand.New(rand.NewSource(1))
			for iteration := 0; iteration < 20; iteration++ {
				random.Shuffle(count, func(i, j int) { tags[i], tags[j] = tags[j], tags[i] })
				beforePointers := append([]*sls.LogTag(nil), backing...)
				beforePairs := tagsTestPairs(backing)
				got := accumulator.getKeyString("project", "store", "topic", "", "source", tags)
				require.Equal(t, want.String(), got)
				require.Equal(t, beforePairs, tagsTestPairs(backing))
				for i := range backing {
					require.Same(t, beforePointers[i], backing[i])
				}
			}
		})
	}
}

var benchmarkProducerKey string

func TestProducerGetKeyStringAllocationsWithoutTags(t *testing.T) {
	accumulator := &LogAccumulator{}
	for _, tc := range []struct {
		name   string
		source string
		hash   string
		tags   []*sls.LogTag
	}{
		{"no-tags", "127.0.0.1", "", nil},
		{"empty-tags", "127.0.0.1", "", make([]*sls.LogTag, 0, 16)},
		{"with-hash", "127.0.0.1", strings.Repeat("a", 32), nil},
		{"long-source", strings.Repeat("s", 4096), "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(1000, func() {
				benchmarkProducerKey = accumulator.getKeyString("test-project", "test-logstore", "topic", tc.hash, tc.source, tc.tags)
			})
			require.Equal(t, float64(1), allocations, "only the returned key's backing buffer should allocate")
		})
	}
}

func TestProducerGetKeyStringAllocationsWithTags(t *testing.T) {
	accumulator := &LogAccumulator{}
	for _, count := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 32, 128} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			tags := make([]*sls.LogTag, count)
			for i := range tags {
				tags[i] = tagsTestTag(fmt.Sprintf("tag-%03d", count-i), "value")
			}
			allocations := testing.AllocsPerRun(1000, func() {
				benchmarkProducerKey = accumulator.getKeyString("test-project", "test-logstore", "topic", "", "127.0.0.1", tags)
			})
			want := float64(1)
			if count > 1 {
				want = 3
			}
			require.Equal(t, want, allocations)
		})
	}
}

func BenchmarkProducerGetKeyString(b *testing.B) {
	manyTags := make([]*sls.LogTag, 16)
	for i := range manyTags {
		manyTags[i] = tagsTestTag(fmt.Sprintf("tag-%02d", len(manyTags)-i), "value")
	}
	largeTags := make([]*sls.LogTag, 32)
	for i := range largeTags {
		largeTags[i] = tagsTestTag(fmt.Sprintf("tag-%02d", len(largeTags)-i), "value")
	}
	for _, tc := range []struct {
		name   string
		source string
		tags   []*sls.LogTag
	}{
		{"no-tags", "127.0.0.1", nil},
		{"empty-tags", "127.0.0.1", []*sls.LogTag{}},
		{"one-tag", "127.0.0.1", manyTags[:1]},
		{"four-tags", "127.0.0.1", manyTags[:4]},
		{"five-tags", "127.0.0.1", manyTags[:5]},
		{"sixteen-tags", "127.0.0.1", manyTags},
		{"seventeen-tags", "127.0.0.1", largeTags[:17]},
		{"thirty-two-tags", "127.0.0.1", largeTags},
		{"long-source", strings.Repeat("s", 256), nil},
		{"long-tag", "127.0.0.1", []*sls.LogTag{tagsTestTag("key", strings.Repeat("v", 1024))}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			accumulator := &LogAccumulator{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkProducerKey = accumulator.getKeyString("test-project", "test-logstore", "topic", "", tc.source, tc.tags)
			}
		})
	}
}
