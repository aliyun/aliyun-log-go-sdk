package main

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
)

func main() {
	// Initialize client
	endpoint := "cn-heyuan.log.aliyuncs.com"
	accessKeyID := os.Getenv("ALIYUN_ACCESS_KEY_ID")
	accessKey := os.Getenv("ALIYUN_ACCESS_KEY_SECRET")
	project := ""
	logstore := ""

	provider := sls.NewStaticCredentialsProvider(accessKeyID, accessKey, "")
	client := sls.CreateNormalInterfaceV2(endpoint, provider)
	client.SetAuthVersion(sls.AuthV4)
	client.SetRegion("cn-heyuan")

	// Example 1: Put a simple object
	fmt.Println("=== Example 1: Put a simple object ===")
	objectName := "test_object_1"
	content := []byte("Hello, this is test content")

	err := client.PutObject(project, logstore, objectName, content, nil)
	if err != nil {
		fmt.Printf("Put object failed: %v\n", err)
		return
	}
	fmt.Println("Put object success!")

	// Get the object back
	resp, err := client.GetObject(project, logstore, objectName)
	if err != nil {
		fmt.Printf("Get object failed: %v\n", err)
		return
	}
	fmt.Printf("Get object success! Body: %s\n", string(resp.Body))
	fmt.Printf("ETag: %s\n", resp.GetETag())
	fmt.Printf("Content-Type: %s\n", resp.GetContentType())

	// Example 2: Put an object with custom headers
	fmt.Println("\n=== Example 2: Put an object with custom headers ===")
	objectName2 := "test_object_2"
	content2 := []byte("Content with metadata")
	headers := map[string]string{
		"Content-Type":       "text/plain",
		"x-log-meta-author":  "test_user",
		"x-log-meta-version": "1.0",
	}

	err = client.PutObject(project, logstore, objectName2, content2, headers)
	if err != nil {
		fmt.Printf("Put object with headers failed: %v\n", err)
		return
	}
	fmt.Println("Put object with headers success!")

	// Get the object back
	resp2, err := client.GetObject(project, logstore, objectName2)
	if err != nil {
		fmt.Printf("Get object failed: %v\n", err)
		return
	}
	fmt.Printf("Get object success! Body: %s\n", string(resp2.Body))
	fmt.Printf("ETag: %s\n", resp2.GetETag())
	fmt.Printf("Content-Type: %s\n", resp2.GetContentType())
	fmt.Printf("x-log-meta-author: %s\n", resp2.Headers["x-log-meta-author"])
	fmt.Printf("x-log-meta-version: %s\n", resp2.Headers["x-log-meta-version"])

	// Example 3: Put an object with Content-MD5
	fmt.Println("\n=== Example 3: Put an object with Content-MD5 ===")
	objectName3 := "test_object_3"
	content3 := []byte("Content with MD5")

	// Calculate MD5
	md5Hash := md5.Sum(content3)
	contentMD5 := base64.StdEncoding.EncodeToString(md5Hash[:])

	headers3 := map[string]string{
		"Content-MD5":  contentMD5,
		"Content-Type": "application/octet-stream",
	}

	err = client.PutObject(project, logstore, objectName3, content3, headers3)
	if err != nil {
		fmt.Printf("Put object with MD5 failed: %v\n", err)
		return
	}
	fmt.Println("Put object with MD5 success!")

	// Get the object back
	resp3, err := client.GetObject(project, logstore, objectName3)
	if err != nil {
		fmt.Printf("Get object failed: %v\n", err)
		return
	}
	fmt.Printf("Get object success! Body: %s\n", string(resp3.Body))
	fmt.Printf("ETag: %s\n", resp3.GetETag())
	fmt.Printf("Content-Type: %s\n", resp3.GetContentType())
}
