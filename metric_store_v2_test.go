package sls_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	sls "github.com/aliyun/aliyun-log-go-sdk"
)

func TestMetricStoreEncryptConfJSON(t *testing.T) {
	store := &sls.MetricStore{
		Name:       "store-1",
		TTL:        30,
		ShardCount: 2,
		EncryptConf: &sls.MetricStoreEncryptConf{
			Enable:      true,
			EncryptType: "m4",
			UserCmkInfo: &sls.MetricStoreEncryptUserCmkConf{
				CmkKeyId: "key-id",
				Arn:      "acs:ram::123456:role/test-role",
				RegionId: "cn-hangzhou",
			},
		},
	}
	data, err := json.Marshal(store)
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &body))
	// the /metricstores API speaks camelCase; snake_case keys are ignored by the server
	require.Contains(t, body, "encryptConf")
	encryptConf, ok := body["encryptConf"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, encryptConf["enable"])
	require.Equal(t, "m4", encryptConf["encryptType"])
	cmkInfo, ok := encryptConf["userCmkInfo"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "key-id", cmkInfo["cmkKeyId"])
	require.Equal(t, "acs:ram::123456:role/test-role", cmkInfo["arn"])
	require.Equal(t, "cn-hangzhou", cmkInfo["regionId"])

	var decoded sls.MetricStore
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.EncryptConf)
	require.True(t, decoded.EncryptConf.Enable)
	require.Equal(t, "m4", decoded.EncryptConf.EncryptType)
	require.Equal(t, "key-id", decoded.EncryptConf.UserCmkInfo.CmkKeyId)
	require.Equal(t, "acs:ram::123456:role/test-role", decoded.EncryptConf.UserCmkInfo.Arn)
	require.Equal(t, "cn-hangzhou", decoded.EncryptConf.UserCmkInfo.RegionId)
}

// A nil EncryptConf must be omitted from the request body so the field is
// simply absent from the serialized request.
func TestMetricStoreEncryptConfOmitted(t *testing.T) {
	store := &sls.MetricStore{
		Name:       "store-1",
		TTL:        30,
		ShardCount: 2,
	}
	data, err := json.Marshal(store)
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &body))
	require.NotContains(t, body, "encryptConf")

	var decoded sls.MetricStore
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Nil(t, decoded.EncryptConf)
}

// Encryption can be requested disabled by carrying the config with Enable=false.
func TestMetricStoreEncryptConfDisable(t *testing.T) {
	store := &sls.MetricStore{
		Name:       "store-1",
		TTL:        30,
		ShardCount: 2,
		EncryptConf: &sls.MetricStoreEncryptConf{
			Enable:      false,
			EncryptType: "m4",
		},
	}
	data, err := json.Marshal(store)
	require.NoError(t, err)
	require.Contains(t, string(data), `"encryptConf"`)
	require.Contains(t, string(data), `"enable":false`)
}
