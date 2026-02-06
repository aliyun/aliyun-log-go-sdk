# Breaking Changes

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
