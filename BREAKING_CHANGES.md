# Breaking Changes

## v0.1.129 (2026-09-08)

### ClientInterface additions

- Added synchronous `UpdateLogStoreLogs` and `DeleteLogStoreLogs` APIs returning `affectedRows`, with support in `Client` and `TokenAutoUpdateClient`. Custom implementations of `ClientInterface` must implement these two methods.
- Added creation-time `EnableModify` request coverage and an E2E test for both enablement paths followed by synchronous update/delete.
- Enable log modification with the existing `EnableLogStoreModify` API or `LogStore.EnableModify` before updating or deleting logs.

## v0.1.128 (2026-09-07)

### MetricStore default change

- The deprecated `CreateMetricStore` path now creates MetricStore V2 by using the `labels` type for `__labels__` in the default `prom` substore.
- Substore key validation now accepts the `labels` type.

## v0.1.124 (2026-08-06)

### ⚠️ Module Change: Store View Routing Checker

- `StoreViewRoutingChecker` and its related APIs have moved from the root `sls` package to the independent `github.com/aliyun/aliyun-log-go-sdk/contrib/storeviewrouting` module.
- The root SDK remains compatible with Go 1.19 and no longer depends on Prometheus; the new module requires Go 1.25 and Prometheus `v0.311.3`.

  Before:

  ```go
  import sls "github.com/aliyun/aliyun-log-go-sdk"

  checker, err := sls.NewStoreViewRoutingChecker(config)
  ```

  After:

  ```go
  import "github.com/aliyun/aliyun-log-go-sdk/contrib/storeviewrouting"

  checker, err := storeviewrouting.NewStoreViewRoutingChecker(config)
  ```

## v0.1.117 (2026-02-26)

### ⚠️ API Change: `GetLogRequest.IsAccurate`

- **Type Change**: The type of `IsAccurate` has been changed from `bool` to `*bool` (pointer).
- **Default Value Change**:
  - **Previous**: Defaults to `false` (via zero-value).
  - **Current**: Defaults to `nil` (unset), which the server now treats as `true`.
- **Impact**: If your implementation relied on the field defaulting to `false`, you must now explicitly set it to `false`, here is a migration example:

    Before:

    ```go
    // Defaulted to false automatically
    req := &GetLogRequest{}
    ```

    After:

    ```go
    isAccurate := false
    req := &GetLogRequest{
      IsAccurate: &isAccurate, // set to false explicitly
    }
    ```
