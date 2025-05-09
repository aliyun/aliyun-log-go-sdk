package sls

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type SubStoreTestSuite struct {
	functionTestSuiteBase
	projectName string
}

func TestSubStoreFunctionTest(t *testing.T) {
	suite.Run(t, new(SubStoreTestSuite))
}

func (s *SubStoreTestSuite) SetupSuite() {
	s.functionTestSuiteBase.init()
	s.projectName = s.createProject()
}

func (s *SubStoreTestSuite) TearDownSuite() {
	s.cleanUpProject(s.projectName)
}

func (s *SubStoreTestSuite) TestSubStore() {
	c := s.getClient().(*Client)
	logstoreName := makeTestLogStoreName()
	err := c.CreateLogStoreV2(s.projectName, &LogStore{
		Name:          logstoreName,
		TTL:           1,
		ShardCount:    2,
		TelemetryType: "Metrics",
	})
	s.Require().NoError(err)

	subStore := &SubStore{}
	subStore.Name = "prom"
	subStore.SortedKeyCount = 2
	subStore.TimeIndex = 2
	subStore.TTL = 1
	subStore.Keys = append(subStore.Keys, SubStoreKey{
		Name: "__name__",
		Type: "text",
	}, SubStoreKey{
		Name: "__labels__",
		Type: "text",
	}, SubStoreKey{
		Name: "__time_nano__",
		Type: "long",
	}, SubStoreKey{
		Name: "__value__",
		Type: "double",
	})
	s.Require().True(subStore.IsValid())
	// create subStore
	err = c.CreateSubStore(s.projectName, logstoreName, subStore)
	s.Require().NoError(err)

	subStoreResp, err := c.GetSubStore(s.projectName, logstoreName, subStore.Name)
	s.Require().NoError(err)
	s.Require().Equal(subStore, subStoreResp)

	err = c.UpdateSubStoreTTL(s.projectName, logstoreName, subStore.TTL+1)
	s.Require().NoError(err)

	ttl, err := c.GetSubStoreTTL(s.projectName, logstoreName)
	s.Require().NoError(err)
	s.Require().Equal(subStore.TTL+1, ttl)

	err = c.DeleteSubStore(s.projectName, logstoreName, subStore.Name)
	s.Require().NoError(err)

	subStoreNames, err := c.ListSubStore(s.projectName, logstoreName)
	s.Require().NoError(err)
	s.Require().Equal(0, len(subStoreNames))
}

type ShardTestSuite struct {
	functionTestSuiteBase
	projectName  string
	logStoreName string
}

func TestShardFunctionTest(t *testing.T) {
	suite.Run(t, new(ShardTestSuite))
}

func (s *ShardTestSuite) SetupSuite() {
	s.functionTestSuiteBase.init()
	s.projectName, s.logStoreName = s.createProjectAndLogStore()
}

func (s *ShardTestSuite) TearDownSuite() {
	s.cleanUpProject(s.projectName)
}

func (s *ShardTestSuite) TestShard() {
	c := s.getClient().(*Client)
	shards, err := c.ListShards(s.projectName, s.logStoreName)
	s.Require().NoError(err)
	s.Require().Greater(len(shards), 0)

	// split
	shardId := shards[0].ShardID
	splitShards, err := c.SplitShard(s.projectName, s.logStoreName, shardId, "")
	s.Require().NoError(err)
	s.Require().Equal(len(splitShards), 3)
	s.Require().Equal(splitShards[0].ShardID, shardId)
	s.Require().Equal(splitShards[0].Status, "readonly")
	s.Require().Equal(splitShards[1].Status, "readwrite")
	s.Require().Equal(splitShards[2].Status, "readwrite")

	// merge
	mergeShards, err := c.MergeShards(s.projectName, s.logStoreName, splitShards[1].ShardID)
	s.Require().NoError(err)
	s.Require().Equal(len(mergeShards), 3)
	s.Require().Equal(mergeShards[0].Status, "readwrite")
	s.Require().Equal(mergeShards[1].Status, "readonly")
	s.Require().Equal(mergeShards[2].Status, "readonly")
}

type CursorTestSuite struct {
	functionTestSuiteBase
	projectName  string
	logStoreName string
}

func TestCursorFunctionTest(t *testing.T) {
	suite.Run(t, new(CursorTestSuite))
}

func (s *CursorTestSuite) SetupSuite() {
	s.functionTestSuiteBase.init()
	s.projectName, s.logStoreName = s.createProjectAndLogStore()
}

func (s *CursorTestSuite) TearDownSuite() {
	s.cleanUpProject(s.projectName)
}

func (s *CursorTestSuite) TestCursor() {
	time.Sleep(time.Second * 3)
	c := s.getClient().(*Client)
	cursor, err := c.GetCursor(s.projectName, s.logStoreName, 0, "begin")
	s.Require().NoError(err)
	s.Require().NotEmpty(cursor)

	_, _, err = c.PullLogs(s.projectName, s.logStoreName, 0, cursor, "", 100)
	s.Require().NoError(err)

	cursorTime, err := c.GetCursorTime(s.projectName, s.logStoreName, 0, cursor)
	s.Require().NoError(err)
	s.Require().NotEmpty(cursorTime)
}
