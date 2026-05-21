//go:build e2e

package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestEventStore(t *testing.T) {
	suite.Run(t, new(EventStoreTestSuite))
}

type EventStoreTestSuite struct {
	suite.Suite
	endpoint        string
	projectName     string
	eventStoreName  string
	accessKeyID     string
	accessKeySecret string
	ttl             int
	shardCnt        int
	client          *Client
}

func (m *EventStoreTestSuite) SetupSuite() {
	m.endpoint = os.Getenv("LOG_TEST_ENDPOINT")
	m.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	m.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	suffix := time.Now().Unix()
	m.projectName = fmt.Sprintf("test-event-store-%d", suffix)
	m.eventStoreName = "test"
	m.ttl = 30
	m.shardCnt = 2
	m.client = &Client{
		Endpoint:        m.endpoint,
		AccessKeyID:     m.accessKeyID,
		AccessKeySecret: m.accessKeySecret,
	}
	_, err := m.client.CreateProject(m.projectName, "test event store")
	m.Require().Nil(err)
	time.Sleep(time.Minute)
}

func (m *EventStoreTestSuite) TearDownSuite() {
	err := m.client.DeleteProject(m.projectName)
	m.Require().Nil(err)
}

func (m *EventStoreTestSuite) TestClient_CreateAndDeleteEventStore() {
	eventStore := &LogStore{
		Name:       m.eventStoreName,
		TTL:        m.ttl,
		ShardCount: m.shardCnt,
	}
	ce := m.client.CreateEventStore(m.projectName, eventStore)
	m.Require().Nil(ce)
	de := m.client.DeleteEventStore(m.projectName, m.eventStoreName)
	m.Require().Nil(de)
}

func (m *EventStoreTestSuite) TestClient_UpdateAndGetEventStore() {
	eventStore := &LogStore{
		Name:       m.eventStoreName,
		TTL:        m.ttl,
		ShardCount: m.shardCnt,
	}
	ce := m.client.CreateEventStore(m.projectName, eventStore)
	m.Require().Nil(ce)
	eventStore, ge := m.client.GetEventStore(m.projectName, m.eventStoreName)
	m.Require().Nil(ge)
	m.Require().Equal(m.eventStoreName, eventStore.Name)
	m.Require().Equal(m.ttl, eventStore.TTL)
	m.Require().Equal(m.shardCnt, eventStore.ShardCount)
	m.Require().Equal(EventStoreTelemetryType, eventStore.TelemetryType)

	eventStore.TTL = 15
	ue := m.client.UpdateEventStore(m.projectName, eventStore)
	m.Require().Nil(ue)
	eventStore1, ge1 := m.client.GetEventStore(m.projectName, m.eventStoreName)
	m.Require().Nil(ge1)
	m.Require().Equal(m.eventStoreName, eventStore1.Name)
	m.Require().Equal(15, eventStore1.TTL)
	m.Require().Equal(EventStoreTelemetryType, eventStore1.TelemetryType)
	de := m.client.DeleteEventStore(m.projectName, m.eventStoreName)
	m.Require().Nil(de)
}
