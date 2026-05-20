//go:build e2e

package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestDashboard(t *testing.T) {
	suite.Run(t, new(DashboardTestSuite))
}

type DashboardTestSuite struct {
	suite.Suite
	endpoint        string
	projectName     string
	logstoreName    string
	accessKeyID     string
	accessKeySecret string
	client          Client
}

func (s *DashboardTestSuite) SetupSuite() {
	s.endpoint = os.Getenv("LOG_TEST_ENDPOINT")
	s.projectName = fmt.Sprintf("test-go-dashboard-%d", time.Now().Unix())
	s.logstoreName = fmt.Sprintf("logstore-%d", time.Now().Unix())
	s.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	s.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	s.client.AccessKeyID = s.accessKeyID
	s.client.AccessKeySecret = s.accessKeySecret
	s.client.Endpoint = s.endpoint
	s.Nil(makeSureLogstoreExist(&s.client, s.projectName, s.logstoreName))
}

func (s *DashboardTestSuite) TearDownSuite() {
	err := s.client.DeleteProject(s.projectName)
	s.Require().Nil(err)
}

func (s *DashboardTestSuite) TestDashboard() {
	// @todo
}

func (s *DashboardTestSuite) TestChart() {
	// @todo
}
