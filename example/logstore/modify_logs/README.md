# Synchronous log updates and deletes

Run from the repository root after setting:

- `SLS_ENDPOINT`: your SLS endpoint, including `https://`.
- `SLS_PROJECT`: an existing test project.
- `ALIBABA_CLOUD_ACCESS_KEY_ID` and `ALIBABA_CLOUD_ACCESS_KEY_SECRET`.
- `ALIBABA_CLOUD_SECURITY_TOKEN`: optional STS security token.

```bash
go run ./example/logstore/modify_logs
```

The example creates its own one-shard Logstore with one-day retention and
`EnableModify: true`. It writes one synthetic record, waits for it to become
queryable, updates its status by query using `UpdateLogStoreLogs`, then deletes
it by row ID using `DeleteLogStoreLogs`. Both operations print `AffectedRows`
and verify their effects through queries. Deferred cleanup deletes the temporary
Logstore, including on errors. A forced process termination can prevent cleanup;
the created Logstore name is printed for manual cleanup if needed.

The account needs Logstore/index management, write, query, and
`log:UpdateLogStoreLogs` / `log:DeleteLogStoreLogs` permissions. The example only
modifies data in its own temporary Logstore.

Both APIs require `From` and `To` (Unix seconds, `[From, To)`). Query and RowID
are alternatives; RowID takes precedence if both are supplied. Query supports
search expressions, not SQL or SPL. Update `Data` is a JSON-encoded string;
`partial` preserves other fields, while `full` replaces the entire record.

API references: [UpdateLogs](https://help.aliyun.com/zh/sls/developer-reference/api-sls-2020-12-30-updatelogs),
[DeleteLogs](https://help.aliyun.com/zh/sls/developer-reference/api-sls-2020-12-30-deletelogs).
