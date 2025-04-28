package sls

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"
)

type InvalidResponseTestSuite struct {
	suite.Suite
	client     *Client
	endpoint   string
	transport  *httpmock.MockTransport
	httpClient *http.Client
}

func TestInvalidResponseTestSuite(t *testing.T) {
	suite.Run(t, new(InvalidResponseTestSuite))
}

func (s *InvalidResponseTestSuite) SetupSuite() {
	httpmock.Activate(s.T())
	s.endpoint = "mock-test-endpoint.aliyuncs.com"
	s.client = CreateNormalInterface(s.endpoint, "testAccessKeyId", "testAccessKeySecret", "").(*Client)
	s.transport = httpmock.NewMockTransport()
	s.httpClient = &http.Client{Transport: s.transport}
	s.client.SetHTTPClient(s.httpClient)
}

func (s *InvalidResponseTestSuite) TearDownSuite() {

}

func (s *InvalidResponseTestSuite) TestInvalidResponse() {
	project, logstore, consumerGroup := "testProject", "testLogstore", "testConsumerGroup"
	path := fmt.Sprintf("http://%s.%s/logstores/%s/consumergroups/%s", project, s.endpoint, logstore, consumerGroup)
	s.transport.RegisterResponder("GET", path,
		httpmock.NewStringResponder(200, `[
		{
			"shard": 0,
			"checkpoint": "MTcxMzg0Njc2OTI4NzU1MjM5Mg==",
			"updateTime": 1723094136866340,
			"consumer": "consumer-1"
		},
		{
			"shard": 1,
			"checkpoint": "MTcxMzc3NjcyNDc1Nzk4MzgwMQ==",
			"updateTime": 1723094138355543,
			"consumer": "consumer-2"
		}]`))
	cp, err := s.client.GetCheckpoint(project, logstore, consumerGroup)
	s.Require().NoError(err)
	s.Require().Equal(2, len(cp))

	invalidResp := `<!DOCTYPE HTML PUBLIC "-//IETF//DTD HTML 2.0//EN">
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr/>Powered by AliyunSLS<hr><center>tengine</center>
</body>
</html>`

	// return 200 and invalid content
	s.transport.Reset()
	s.transport.RegisterResponder("GET", path,
		httpmock.NewStringResponder(200, invalidResp))
	cp, err = s.client.GetCheckpoint(project, logstore, consumerGroup)
	s.Require().Error(err)
	s.Require().Nil(cp)
	slsErr, ok := err.(*Error)
	if ok {
		s.Require().LessOrEqual(slsErr.HTTPCode, int32(0))
	} else {
		s.Require().False(ok)
		s.Require().Nil(slsErr)
	}

	// return not 200 and invalid content
	s.transport.Reset()
	s.transport.RegisterResponder("GET", path,
		httpmock.NewStringResponder(400, invalidResp))
	cp, err = s.client.GetCheckpoint(project, logstore, consumerGroup)
	s.Require().Error(err)
	s.Require().Nil(cp)
	slsErr, ok = err.(*Error)
	if ok {
		s.Require().LessOrEqual(slsErr.HTTPCode, int32(0))
	} else {
		s.Require().False(ok)
		s.Require().Nil(slsErr)
	}
}
