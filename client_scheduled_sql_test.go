//go:build e2e

package sls

import (
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestScheduledSQL(t *testing.T) {
	suite.Run(t, new(ScheduledSQLTestSuite))
}

type ScheduledSQLTestSuite struct {
	suite.Suite
	endpoint           string
	accessKeyID        string
	accessKeySecret    string
	projectName        string
	sourceLogStore     string
	targetLogStoreName string
	scheduledSQLName   string
	displayName        string
	client             *Client
}

func (s *ScheduledSQLTestSuite) SetupSuite() {
	s.endpoint = os.Getenv("LOG_TEST_ENDPOINT")
	s.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	s.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	suffix := time.Now().Unix()
	s.projectName = fmt.Sprintf("test-scheduled-sql-%d", suffix)
	s.sourceLogStore = "test-source"
	s.targetLogStoreName = "test-target"
	s.scheduledSQLName = fmt.Sprintf("schedulesql-%d", suffix)
	s.displayName = fmt.Sprintf("display-%d", suffix)
	s.client = &Client{
		Endpoint:        s.endpoint,
		AccessKeyID:     s.accessKeyID,
		AccessKeySecret: s.accessKeySecret,
	}
	s.setUp()
}

func (s *ScheduledSQLTestSuite) TearDownSuite() {
	err := s.client.DeleteProject(s.projectName)
	s.Require().Nil(err)
}

func (s *ScheduledSQLTestSuite) TestClient_CreateAndDeleteScheduledSQL() {
	ce := s.client.CreateScheduledSQL(s.projectName, s.getScheduleSQL("111"))
	s.Require().Nil(ce)
	de := s.client.DeleteScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(de)
}

func (s *ScheduledSQLTestSuite) TestClient_UpdateAndGetScheduledSQL() {
	ce := s.client.CreateScheduledSQL(s.projectName, s.getScheduleSQL("111"))
	s.Require().Nil(ce)
	scheduledSQL, ge := s.client.GetScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(ge)
	s.Require().Equal(s.scheduledSQLName, scheduledSQL.Name)
	s.Require().Equal("111", scheduledSQL.Description)
	ue := s.client.UpdateScheduledSQL(s.projectName, s.getScheduleSQL("222"))
	s.Require().Nil(ue)
	scheduledSQL2, ge2 := s.client.GetScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(ge2)
	s.Require().Equal(s.scheduledSQLName, scheduledSQL2.Name)
	s.Require().Equal("222", scheduledSQL2.Description)
	de := s.client.DeleteScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(de)
}

func (s *ScheduledSQLTestSuite) TestClient_ListScheduledSQL() {
	ce := s.client.CreateScheduledSQL(s.projectName, s.getScheduleSQL("111"))
	s.Require().Nil(ce)
	scheduledsqls, total, count, le := s.client.ListScheduledSQL(s.projectName, "", "", 0, 10)
	s.Require().Nil(le)
	s.Require().Equal(1, len(scheduledsqls))
	s.Require().Equal(1, total)
	s.Require().Equal(1, count)
	de := s.client.DeleteScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(de)
}

func (s *ScheduledSQLTestSuite) TestClient_ScheduledSQLInstances() {
	prepareData(s)
	ce := s.client.CreateScheduledSQL(s.projectName, s.getScheduleSQL("111"))
	s.Require().Nil(ce)
	time.Sleep(time.Minute * 5)
	status := &InstanceStatus{
		FromTime: time.Now().Unix() - 600,
		ToTime:   time.Now().Unix() + 600,
		Offset:   0,
		Size:     3,
		State:    ScheduledSQL_SUCCEEDED,
	}
	instances, total, count, le := s.client.ListScheduledSQLJobInstances(s.projectName, s.scheduledSQLName, status)
	s.Require().Nil(le)
	s.Require().Equal(3, len(instances))
	s.Require().Equal(int64(3), count)
	s.Require().Equal(true, total > 3)
	instance := instances[0]
	jobInstance, ge := s.client.GetScheduledSQLJobInstance(s.projectName, s.scheduledSQLName, instance.InstanceId, true)
	s.Require().Nil(ge)
	s.Require().Equal(ScheduledSQL_SUCCEEDED, jobInstance.State)
	me := s.client.ModifyScheduledSQLJobInstanceState(s.projectName, s.scheduledSQLName, instance.InstanceId, ScheduledSQL_RUNNING)
	s.Require().Nil(me)
	jobInstance2, ge2 := s.client.GetScheduledSQLJobInstance(s.projectName, s.scheduledSQLName, instance.InstanceId, true)
	s.Require().Nil(ge2)
	s.Require().Equal(true, ScheduledSQL_SUCCEEDED != jobInstance2.State)
	time.Sleep(time.Second * 5)
	de := s.client.DeleteScheduledSQL(s.projectName, s.scheduledSQLName)
	s.Require().Nil(de)
}

func prepareData(s *ScheduledSQLTestSuite) {
	for loggroupIdx := 0; loggroupIdx < 10; loggroupIdx++ {
		var logs []*Log
		for logIdx := 0; logIdx < 100; logIdx++ {
			var content []*LogContent
			for colIdx := 0; colIdx < 10; colIdx++ {
				if colIdx == 0 {
					content = append(content, &LogContent{
						Key:   proto.String(fmt.Sprintf("col_%d", colIdx)),
						Value: proto.String(fmt.Sprintf("%d", rand.Intn(10000000))),
					})
				} else {
					content = append(content, &LogContent{
						Key:   proto.String(fmt.Sprintf("col_%d", colIdx)),
						Value: proto.String(fmt.Sprintf("logGroup idx: %d, log idx: %d, col idx: %d, value: %d", loggroupIdx, logIdx, colIdx, rand.Intn(10000000))),
					})
				}
			}
			log := &Log{
				Time:     proto.Uint32(uint32(time.Now().Unix())),
				Contents: content,
			}
			logs = append(logs, log)
		}
		logGroup := &LogGroup{
			Topic:  proto.String("test"),
			Source: proto.String("10.238.222.116"),
			Logs:   logs,
		}
		err := s.client.PutLogs(s.projectName, s.sourceLogStore, logGroup)
		s.Require().Nil(err)
	}
	time.Sleep(time.Second * 5)
}

func (s *ScheduledSQLTestSuite) getScheduleSQL(des string) *ScheduledSQL {
	return &ScheduledSQL{
		Name:        s.scheduledSQLName,
		DisplayName: s.displayName,
		Description: des,
		Status:      ENABLED,
		Configuration: &ScheduledSQLConfiguration{
			SourceLogStore:      s.sourceLogStore,
			DestProject:         s.projectName,
			DestEndpoint:        s.endpoint,
			DestLogStore:        s.targetLogStoreName,
			Script:              "*|SELECT COUNT(col_0) as value_count",
			SqlType:             SEARCH_QUERY,
			ResourcePool:        DEFAULT,
			RoleArn:             os.Getenv("LOG_TEST_ROLE_ARN"),
			DestRoleArn:         os.Getenv("LOG_TEST_ROLE_ARN"),
			FromTimeExpr:        "@m-1m",
			ToTimeExpr:          "@m",
			MaxRunTimeInSeconds: 60,
			MaxRetries:          20,
			FromTime:            time.Now().Unix() - 300,
			ToTime:              time.Now().Unix() + 300,
			DataFormat:          LOG_TO_LOG,
			Parameters:          nil,
		},
		Schedule: &Schedule{
			Type:      "FixedRate",
			Interval:  "1m",
			Delay:     10,
			DayOfWeek: 0,
			Hour:      0,
		},
		CreateTime:       0,
		LastModifiedTime: 0,
		Type:             SCHEDULED_SQL_JOB,
	}
}

func (s *ScheduledSQLTestSuite) setUp() {
	_, ce := s.client.CreateProject(s.projectName, "test scheduled sql")
	s.Require().Nil(ce)
	time.Sleep(time.Second * 60)
	cle := s.client.CreateLogStore(s.projectName, s.sourceLogStore, 3, 2, true, 4)
	s.Require().Nil(cle)
	cle2 := s.client.CreateLogStore(s.projectName, s.targetLogStoreName, 3, 2, true, 4)
	s.Require().Nil(cle2)
	cie := s.client.CreateIndex(s.projectName, s.sourceLogStore, Index{
		Keys: map[string]IndexKey{
			"col_0": {
				Token:         []string{" "},
				DocValue:      true,
				CaseSensitive: false,
				Type:          "long",
			},
			"col_1": {
				Token:         []string{",", ":", " "},
				DocValue:      true,
				CaseSensitive: false,
				Type:          "text",
			},
		},
	})
	s.Require().Nil(cie)
	time.Sleep(time.Second * 60)
}
