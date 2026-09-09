# Breaking Changes

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
