//go:build e2e

package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	env "github.com/Netflix/go-env"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	"github.com/stretchr/testify/assert"

	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
)

type testCredentials struct {
	AccessKeyID     string `env:"LOG_TEST_ACCESS_KEY_ID"`
	AccessKeySecret string `env:"LOG_TEST_ACCESS_KEY_SECRET"`
	RoleArn         string `env:"LOG_TEST_ROLE_ARN"`
	Endpoint        string `env:"LOG_STS_TEST_ENDPOINT"`
}

func getStsClient(c *testCredentials) (*sts.Client, error) {
	conf := &openapi.Config{
		AccessKeyId:     &c.AccessKeyID,
		AccessKeySecret: &c.AccessKeySecret,
		Endpoint:        &c.Endpoint,
	}
	return sts.NewClient(conf)
}

// set env virables before test
func TestStsToken(t *testing.T) {
	testutil.RequireE2E(t)
	c := testCredentials{}
	_, err := env.UnmarshalFromEnviron(&c)
	if err != nil {
		assert.Fail(t, "set ACCESS_KEY_ID/ACCESS_KEY_SECRET in environment first")
	}
	client, err := getStsClient(&c)
	assert.NoError(t, err)
	callCnt := 0
	updateFunc := func() (string, string, string, time.Time, error) {
		callCnt++
		name := "test-go-sdk-session"
		req := &sts.AssumeRoleRequest{
			RoleArn:         &c.RoleArn,
			RoleSessionName: &name,
		}
		resp, err := client.AssumeRole(req)
		assert.NoError(t, err)
		cred := resp.Body.Credentials
		e := cred.Expiration
		assert.NotNil(t, e)
		ex, err := time.Parse(time.RFC3339, *e)
		assert.NoError(t, err)
		return *cred.AccessKeyId, *cred.AccessKeySecret, *cred.SecurityToken, ex, nil
	}
	provider := NewUpdateFuncProviderAdapter(updateFunc)

	cred1, err := provider.GetCredentials()
	assert.NoError(t, err)
	assert.Equal(t, 1, callCnt)
	// fetch again, updateFunc not called, use cache
	cred2, err := provider.GetCredentials()
	assert.NoError(t, err)
	assert.EqualValues(t, cred1, cred2)
	assert.Equal(t, 1, callCnt)
	endpoint := os.Getenv("LOG_TEST_ENDPOINT")
	project := os.Getenv("LOG_TEST_PROJECT")
	client2 := CreateNormalInterfaceV2(endpoint, provider)
	res, err := client2.CheckProjectExist(project)
	assert.NoError(t, err)
	fmt.Println(res)
}

func TestTokenAutoUpdateClient(t *testing.T) {
	testutil.RequireE2E(t)
	c := testCredentials{}
	_, err := env.UnmarshalFromEnviron(&c)
	if err != nil {
		assert.Fail(t, "set ACCESS_KEY_ID/ACCESS_KEY_SECRET in environment first")
	}
	client, err := getStsClient(&c)
	assert.NoError(t, err)
	endpoint := os.Getenv("LOG_TEST_ENDPOINT")
	project := os.Getenv("LOG_TEST_PROJECT")
	callCnt := 0
	updateFunc := func() (string, string, string, time.Time, error) {
		callCnt++
		name := "test-go-sdk-session"
		req := &sts.AssumeRoleRequest{
			RoleArn:         &c.RoleArn,
			RoleSessionName: &name,
		}
		resp, err := client.AssumeRole(req)
		assert.NoError(t, err)
		cred := resp.Body.Credentials
		e := cred.Expiration
		assert.NotNil(t, e)
		ex, err := time.Parse(time.RFC3339, *e)
		assert.NoError(t, err)
		return *cred.AccessKeyId, *cred.AccessKeySecret, *cred.SecurityToken, ex, nil
	}
	done := make(chan struct{})
	updateClient, err := CreateTokenAutoUpdateClient(endpoint, updateFunc, done)
	assert.NoError(t, err)
	res, err := updateClient.CheckProjectExist(project)
	assert.NoError(t, err)
	fmt.Println(res)
}
