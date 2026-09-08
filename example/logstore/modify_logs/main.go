// This example creates a temporary Logstore, updates and deletes a synthetic
// log, and deletes the Logstore on exit. See README.md for configuration.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	for _, key := range []string{"SLS_ENDPOINT", "SLS_PROJECT", "ALIBABA_CLOUD_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_SECRET"} {
		if os.Getenv(key) == "" {
			return fmt.Errorf("set %s before running this example", key)
		}
	}
	project := os.Getenv("SLS_PROJECT")
	client := sls.CreateNormalInterfaceV2(os.Getenv("SLS_ENDPOINT"), sls.NewStaticCredentialsProvider(
		os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"), os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"), os.Getenv("ALIBABA_CLOUD_SECURITY_TOKEN"),
	))
	client.SetHTTPClient(&http.Client{Timeout: 30 * time.Second})
	store := fmt.Sprintf("go-modify-example-%d", time.Now().UnixNano())
	if err = client.CreateLogStoreV2(project, &sls.LogStore{Name: store, TTL: 1, ShardCount: 1, EnableModify: true}); err != nil {
		return err
	}
	fmt.Println("Created", store, "with enableModify=true")
	defer func() {
		if cleanupErr := client.DeleteLogStore(project, store); cleanupErr != nil {
			if err == nil {
				err = fmt.Errorf("delete temporary logstore %s: %w", store, cleanupErr)
			} else {
				fmt.Fprintf(os.Stderr, "Cleanup %s failed: %v\n", store, cleanupErr)
			}
		} else {
			fmt.Println("Cleaned up", store)
		}
	}()
	if err = client.CreateIndex(project, store, *sls.CreateDefaultIndex()); err != nil {
		return err
	}
	// Wait for the index configuration to reach the data plane before writing.
	time.Sleep(10 * time.Second)
	now := time.Now().Unix()
	if err = client.PutLogs(project, store, &sls.LogGroup{Logs: []*sls.Log{{
		Time: proto.Uint32(uint32(now)), Contents: []*sls.LogContent{
			{Key: proto.String("message"), Value: proto.String("SDK example")},
			{Key: proto.String("status"), Value: proto.String("before")},
		},
	}}}); err != nil {
		return err
	}

	// From and To are required even for RowID operations. Their relationship is
	// validated by the service. This example owns the entire temporary store.
	request := &sls.GetLogRequest{From: now - 1, To: now + 2, Query: "*", Lines: 100}
	rows, err := waitForLogs(client, project, store, request, "before")
	if err != nil {
		return err
	}
	rowID := rows[0]["__rowid__"]
	if rowID == "" {
		return fmt.Errorf("query did not return __rowid__")
	}

	// Query is an index search expression, not SQL or SPL. If RowID is also set,
	// the service gives RowID precedence. Data is a JSON string, not a JSON object.
	updated, err := client.UpdateLogStoreLogs(project, store, &sls.UpdateLogStoreLogsRequest{
		From: request.From, To: request.To, Query: "*", UpdateMode: "partial", Data: `{"status":"after"}`,
	})
	if err != nil {
		return err
	}
	fmt.Println("Updated affectedRows:", updated.AffectedRows)
	if updated.AffectedRows != 1 {
		return fmt.Errorf("expected one updated row, got %d", updated.AffectedRows)
	}
	if _, err = waitForLogs(client, project, store, request, "after"); err != nil {
		return err
	}

	deleted, err := client.DeleteLogStoreLogs(project, store, &sls.DeleteLogStoreLogsRequest{
		From: request.From, To: request.To, RowID: rowID,
	})
	if err != nil {
		return err
	}
	fmt.Println("Deleted affectedRows:", deleted.AffectedRows)
	if deleted.AffectedRows != 1 {
		return fmt.Errorf("expected one deleted row, got %d", deleted.AffectedRows)
	}
	_, err = waitForLogs(client, project, store, request, "")
	return err
}

func waitForLogs(client sls.ClientInterface, project, store string, req *sls.GetLogRequest, status string) ([]map[string]string, error) {
	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); time.Sleep(3 * time.Second) {
		resp, err := client.GetLogsV2(project, store, req)
		if err != nil {
			return nil, err
		}
		if resp.Progress != "Complete" {
			continue
		}
		if status == "" && len(resp.Logs) == 0 {
			return resp.Logs, nil
		}
		if status != "" && len(resp.Logs) == 1 && resp.Logs[0]["status"] == status {
			return resp.Logs, nil
		}
	}
	return nil, fmt.Errorf("timed out waiting for logs with status %q", status)
}
