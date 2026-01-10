package main

import (
	"fmt"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
)

func main() {
	// Initialize client
	endpoint := "cn-hangzhou.log.aliyuncs.com"
	accessKeyID := os.Getenv("ALIYUN_ACCESS_KEY_ID")
	accessKey := os.Getenv("ALIYUN_ACCESS_KEY_SECRET")
	project := ""
	logstore := ""

	provider := sls.NewStaticCredentialsProvider(accessKeyID, accessKey, "")
	client := sls.CreateNormalInterfaceV2(endpoint, provider)
	client.SetAuthVersion(sls.AuthV4)
	client.SetRegion("cn-hangzhou")

	fmt.Println("=== Multimodal Configuration Example ===\n")

	// Example 1: Get current multimodal configuration
	fmt.Println("=== Example 1: Get multimodal configuration ===")
	resp, err := client.GetLogStoreMultimodalConfiguration(project, logstore)
	if err != nil {
		fmt.Printf("Failed to get multimodal configuration: %v\n", err)
	} else {
		fmt.Printf("Current Status: %s\n", resp.Status)
		if resp.AnonymousWrite != "" {
			fmt.Printf("Anonymous Write: %s\n", resp.AnonymousWrite)
		}
	}
	fmt.Println()

	// Example 2: Enable multimodal configuration
	fmt.Println("=== Example 2: Enable multimodal configuration ===")
	err = client.PutLogStoreMultimodalConfiguration(project, logstore, "Enabled")
	if err != nil {
		fmt.Printf("Failed to enable multimodal configuration: %v\n", err)
	} else {
		fmt.Println("Successfully enabled multimodal configuration")
	}
	fmt.Println()

	// Example 3: Enable with anonymous write
	fmt.Println("=== Example 3: Enable multimodal configuration with anonymous write ===")
	err = client.PutLogStoreMultimodalConfiguration(project, logstore, "Enabled", "Enabled")
	if err != nil {
		fmt.Printf("Failed to enable multimodal configuration with anonymous write: %v\n", err)
	} else {
		fmt.Println("Successfully enabled multimodal configuration with anonymous write")
	}
	fmt.Println()

	// Example 4: Verify the configuration
	fmt.Println("=== Example 4: Verify the configuration ===")
	resp, err = client.GetLogStoreMultimodalConfiguration(project, logstore)
	if err != nil {
		fmt.Printf("Failed to get multimodal configuration: %v\n", err)
	} else {
		fmt.Printf("Updated Status: %s\n", resp.Status)
		if resp.AnonymousWrite != "" {
			fmt.Printf("Updated Anonymous Write: %s\n", resp.AnonymousWrite)
		}
	}
	fmt.Println()

	// Example 5: Disable anonymous write
	fmt.Println("=== Example 5: Disable anonymous write ===")
	err = client.PutLogStoreMultimodalConfiguration(project, logstore, "Enabled", "Disabled")
	if err != nil {
		fmt.Printf("Failed to disable anonymous write: %v\n", err)
	} else {
		fmt.Println("Successfully disabled anonymous write")
	}
	fmt.Println()

	// Example 6: Disable multimodal configuration
	fmt.Println("=== Example 6: Disable multimodal configuration ===")
	err = client.PutLogStoreMultimodalConfiguration(project, logstore, "Disabled")
	if err != nil {
		fmt.Printf("Failed to disable multimodal configuration: %v\n", err)
	} else {
		fmt.Println("Successfully disabled multimodal configuration")
	}
	fmt.Println()

	fmt.Println("=== All examples completed ===")
}
