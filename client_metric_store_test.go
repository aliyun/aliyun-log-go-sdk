package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestMetricStore(t *testing.T) {
	suite.Run(t, new(MetricStoreTestSuite))
}

type MetricStoreTestSuite struct {
	suite.Suite
	endpoint        string
	projectName     string
	metricStoreName string
	accessKeyID     string
	accessKeySecret string
	ttl             int
	shardCnt        int
	client          *Client
}

func (m *MetricStoreTestSuite) SetupSuite() {
	m.endpoint = os.Getenv("LOG_TEST_ENDPOINT")
	m.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	m.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	suffix := time.Now().Unix()
	m.projectName = fmt.Sprintf("test-metric-store-%d", suffix)
	m.metricStoreName = "test"
	m.ttl = 30
	m.shardCnt = 2
	m.client = &Client{
		Endpoint:        m.endpoint,
		AccessKeyID:     m.accessKeyID,
		AccessKeySecret: m.accessKeySecret,
	}
	_, err := m.client.CreateProject(m.projectName, "test metric store")
	m.Require().Nil(err)
	time.Sleep(time.Minute)
}

func (m *MetricStoreTestSuite) TearDownSuite() {
	err := m.client.DeleteProject(m.projectName)
	m.Require().Nil(err)
}

func (m *MetricStoreTestSuite) TestClient_CreateAndDeleteMetricStore() {
	metricStore := &MetricStore{
		Name:       m.metricStoreName,
		TTL:        m.ttl,
		ShardCount: m.shardCnt,
	}
	ce := m.client.CreateMetricStoreV2(m.projectName, metricStore)
	m.Require().Nil(ce)
	de := m.client.DeleteMetricStoreV2(m.projectName, m.metricStoreName)
	m.Require().Nil(de)
}

func (m *MetricStoreTestSuite) TestClient_UpdateAndGetMetricStore() {
	metricStore1 := &MetricStore{
		Name:       m.metricStoreName,
		TTL:        m.ttl,
		ShardCount: m.shardCnt,
	}
	ce := m.client.CreateMetricStoreV2(m.projectName, metricStore1)
	m.Require().Nil(ce)
	metricStore, ge := m.client.GetMetricStoreV2(m.projectName, m.metricStoreName)
	m.Require().Nil(ge)
	m.Require().Equal(m.metricStoreName, metricStore.Name)
	m.Require().Equal(m.ttl, metricStore.TTL)
	m.Require().Equal(m.shardCnt, metricStore.ShardCount)

	metricStore1.TTL = 15
	ue := m.client.UpdateMetricStoreV2(m.projectName, metricStore1)
	m.Require().Nil(ue)
	metricStore2, ge2 := m.client.GetMetricStoreV2(m.projectName, m.metricStoreName)
	m.Require().Nil(ge2)
	m.Require().Equal(m.metricStoreName, metricStore2.Name)
	m.Require().Equal(15, metricStore2.TTL)
	de := m.client.DeleteMetricStoreV2(m.projectName, m.metricStoreName)
	m.Require().Nil(de)
}

func TestClient_MetricStoreMeteringMode(t *testing.T) {
	client := CreateNormalInterface(os.Getenv("LOG_TEST_ENDPOINT"), os.Getenv("LOG_TEST_ACCESS_KEY_ID"), os.Getenv("LOG_TEST_ACCESS_KEY_SECRET"), "")
	projectName := os.Getenv("LOG_TEST_PROJECT_NAME")
	metricStoreName := os.Getenv("LOG_TEST_METRIC_STORE_NAME")
	// 获取初始计量模式
	res, err := client.GetMetricStoreMeteringMode(projectName, metricStoreName)
	if err != nil {
		t.Fatalf("获取计量模式失败: %v", err)
	}
	initialMode := res.MeteringMode
	fmt.Printf("Initial metering mode: %s\n", initialMode)

	// 切换到 ChargeByDataIngest
	err = client.UpdateMetricStoreMeteringMode(projectName, metricStoreName, CHARGE_BY_FUNCTION)
	if err != nil {
		t.Fatalf("更新计量模式失败: %v", err)
	}
	res, err = client.GetMetricStoreMeteringMode(projectName, metricStoreName)
	if err != nil {
		t.Fatalf("获取计量模式失败: %v", err)
	}
	fmt.Printf("Changed to: %s\n", res.MeteringMode)
}
