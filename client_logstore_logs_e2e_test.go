//go:build e2e

package sls_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

// TestLogStoreLogsE2E creates and cleans up its own stores in LOG_TEST_PROJECT.
func TestLogStoreLogsE2E(t *testing.T) {
	testutil.RequireE2E(t)
	cfg, _ := testutil.LoadE2EConfig()
	client := sls.CreateNormalInterface(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret, "")
	client.SetHTTPClient(&http.Client{Timeout: 30 * time.Second})
	for _, atCreation := range []bool{true, false} {
		t.Run(fmt.Sprintf("enableAtCreation=%t", atCreation), func(t *testing.T) {
			if !atCreation && os.Getenv("LOG_TEST_ENABLE_EXISTING_MODIFY") != "true" {
				t.Skip("requires service support and LOG_TEST_ENABLE_EXISTING_MODIFY=true")
			}
			store := fmt.Sprintf("go-log-mutations-%d", time.Now().UnixNano())
			require.NoError(t, client.CreateLogStoreV2(cfg.Project, &sls.LogStore{
				Name: store, TTL: 1, ShardCount: 1, EnableModify: atCreation,
			}))
			t.Logf("created temporary logstore %s", store)
			t.Cleanup(func() { require.NoError(t, client.DeleteLogStore(cfg.Project, store)) })
			if !atCreation {
				require.NoError(t, client.EnableLogStoreModify(cfg.Project, store))
			}
			require.Eventually(t, func() bool {
				got, err := client.GetLogStore(cfg.Project, store)
				return err == nil && got.EnableModify
			}, 3*time.Minute, 3*time.Second)
			index := sls.CreateDefaultIndex()
			index.Keys = map[string]sls.IndexKey{"id": {Type: "text", Token: []string{}}, "status": {Type: "text", Token: []string{}}}
			require.NoError(t, client.CreateIndex(cfg.Project, store, *index))
			_, err := client.GetIndex(cfg.Project, store)
			require.NoError(t, err)
			// Allow the index configuration to reach the data plane before writing.
			time.Sleep(10 * time.Second)
			now := time.Now().Unix()
			group := &sls.LogGroup{}
			for _, id := range []string{"query", "row"} {
				group.Logs = append(group.Logs, &sls.Log{Time: proto.Uint32(uint32(now)), Contents: []*sls.LogContent{
					{Key: proto.String("id"), Value: proto.String(id)},
					{Key: proto.String("status"), Value: proto.String("before")},
				}})
			}
			require.NoError(t, client.PutLogs(cfg.Project, store, group))
			query := func() (*sls.GetLogsResponse, error) {
				return client.GetLogsV2(cfg.Project, store, &sls.GetLogRequest{From: now - 1, To: now + 2, Query: "*", Lines: 100})
			}
			var rows []map[string]string
			require.Eventually(t, func() bool {
				got, err := query()
				if err != nil || got.Progress != "Complete" || len(got.Logs) != 2 {
					return false
				}
				rows = got.Logs
				return true
			}, 3*time.Minute, 3*time.Second)
			for _, row := range rows {
				req := &sls.UpdateLogStoreLogsRequest{From: now - 1, To: now + 2, UpdateMode: "partial", Data: `{"status":"after"}`}
				if row["id"] == "query" {
					req.Query = "id:query"
				} else {
					require.NotEmpty(t, row["__rowid__"])
					req.RowID = row["__rowid__"]
				}
				updated, err := client.UpdateLogStoreLogs(cfg.Project, store, req)
				require.NoError(t, err)
				require.Equal(t, int64(1), updated.AffectedRows)
				t.Logf("updated %s: affectedRows=%d", row["id"], updated.AffectedRows)
			}
			require.Eventually(t, func() bool {
				got, err := query()
				if err != nil || got.Progress != "Complete" || len(got.Logs) != 2 {
					return false
				}
				for _, row := range got.Logs {
					if row["status"] != "after" {
						return false
					}
				}
				rows = got.Logs
				return true
			}, 3*time.Minute, 3*time.Second)
			for _, row := range rows {
				req := &sls.DeleteLogStoreLogsRequest{From: now - 1, To: now + 2}
				if row["id"] == "query" {
					req.Query = "id:query"
				} else {
					require.NotEmpty(t, row["__rowid__"])
					req.RowID = row["__rowid__"]
				}
				deleted, err := client.DeleteLogStoreLogs(cfg.Project, store, req)
				require.NoError(t, err)
				require.Equal(t, int64(1), deleted.AffectedRows)
				t.Logf("deleted %s: affectedRows=%d", row["id"], deleted.AffectedRows)
			}
			require.Eventually(t, func() bool {
				got, err := query()
				return err == nil && got.Progress == "Complete" && len(got.Logs) == 0
			}, 3*time.Minute, 3*time.Second)
		})
	}
}
