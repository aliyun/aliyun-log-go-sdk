# Testing aliyun-log-go-sdk

Tests in this repo are split into two classes:

- **Unit tests** — run with the default build (`go test ./...`). They never touch the network. Safe to run in any environment, including CI on every push.
- **E2E (functional) tests** — guarded behind the `e2e` build tag. They require a real Alibaba Cloud SLS endpoint and access keys. Run them with `go test -tags=e2e ./...`.

## Running unit tests

```bash
go test ./...
```

No environment variables required. All HTTP traffic in unit tests is mocked.

## Running E2E tests

Set the following environment variables, then build with `-tags=e2e`:

```bash
export LOG_TEST_ENDPOINT=cn-hangzhou.log.aliyuncs.com
export LOG_TEST_ACCESS_KEY_ID=...
export LOG_TEST_ACCESS_KEY_SECRET=...
export LOG_TEST_PROJECT=my-test-project
export LOG_TEST_LOGSTORE=my-test-logstore
# Optional, depending on which suites you run:
export LOG_TEST_REGION=cn-hangzhou
export LOG_TEST_ROLE_ARN=acs:ram::...
export LOG_TEST_CMK_ID=...
export LOG_TEST_CMK_ENDPOINT=...
export LOG_TEST_METRIC_STORE_NAME=...
export LOG_TEST_STORE_VIEW_PROJECT=...

go test -tags=e2e ./...
```

When `LOG_TEST_*` is missing, e2e suites call `testutil.RequireE2E(t)`, which calls `t.Skip` rather than failing.

## Mock infrastructure (`internal/testutil/`)

Reusable helpers for writing new unit tests that exercise the HTTP boundary without a real network call.

### Building a mocked client

```go
import (
    "github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
    "github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
)

transport := testutil.NewMockTransport()
testutil.RegisterJSON(t, transport, "POST", `=~^https://.+\.example\.com/logstores/.+/shards/lb$`,
    200, map[string]any{}, nil)

client := clienthelper.NewMockedClient(transport)
// ... call client.PutLogs / GetLogs / etc.
```

`testutil.NewMockTransport` wraps `github.com/jarcoal/httpmock`. Register stubs with `RegisterJSON` (success) or `RegisterError` (SLS error envelope).

### Skipping when no real endpoint is configured

```go
func TestSomethingE2E(t *testing.T) {
    cfg := testutil.RequireE2E(t)  // skips if LOG_TEST_* env is empty
    client := sls.CreateNormalInterface(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret, "")
    // ...
}
```

## Naming convention

- File `xxx_test.go` (no build tag) → unit test, runs by default.
- File `xxx_e2e_test.go` with `//go:build e2e` first line → e2e test, runs only with `-tags=e2e`.

When extracting an e2e suite from a mixed file, leave the unit-only logic in the original `xxx_test.go` and put the e2e suite in a sibling `xxx_e2e_test.go`.

## CI

`.github/workflows/go.yml` runs the unit job on every push/PR to `master`. The e2e job runs only on manual workflow dispatch and consumes `LOG_TEST_*` from repository secrets.

## Known limitations

- `TestSignerV4Suite/TestSignV1Case{1,2}` in `signature_v4_test.go` is currently failing on master (expected `"SLS"` prefix vs production `"LOG"` prefix). Pre-existing — not introduced by the unit/e2e split.
- The `cgo/` subpackage is not built by default. It depends on `github.com/DataDog/zstd` and requires a working CGO toolchain. Skip it in environments without one.
