//go:build e2e

package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestResourceRecord(t *testing.T) {
	suite.Run(t, new(ResourceRecordTestSuite))
}

type ResourceRecordTestSuite struct {
	suite.Suite
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	client          Client
	recordId        string
	tagName         string
	resourceName    string
}

func (s *ResourceRecordTestSuite) SetupSuite() {
	s.endpoint = "cn-heyuan.log.aliyuncs.com"
	s.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	s.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	s.client.AccessKeyID = s.accessKeyID
	s.client.AccessKeySecret = s.accessKeySecret
	s.client.Endpoint = s.endpoint
	s.resourceName = fmt.Sprintf("user.test_resource_%d", time.Now().Unix())
	s.recordId = "test_record_1"
	s.tagName = "test record"

	_ = s.client.DeleteResource(s.resourceName)
	_ = s.createResource()
}

func (s *ResourceRecordTestSuite) createResource() error {
	rs := &ResourceSchema{
		Schema: []*ResourceSchemaItem{
			&ResourceSchemaItem{
				Column:   "col1",
				Desc:     "col1 desc",
				ExtInfo:  map[string]string{},
				Required: true,
				Type:     "string",
			},
			&ResourceSchemaItem{
				Column:   "col2",
				Desc:     "col2 desc",
				ExtInfo:  "optional",
				Required: true,
				Type:     "string",
			},
		},
	}
	customResource := new(Resource)
	customResource.Type = ResourceTypeUserDefine
	customResource.Name = s.resourceName
	customResource.Schema = rs.ToString()
	customResource.Description = "user test resource 1 descc"
	return s.client.CreateResource(customResource)
}

func (s *ResourceRecordTestSuite) TearDownSuite() {
	err := s.client.DeleteResource(s.resourceName)
	s.Require().Nil(err)
}

func (s *ResourceRecordTestSuite) createResourceRecord() error {
	customResourceRecord := new(ResourceRecord)
	customResourceRecord.Id = s.recordId
	customResourceRecord.Tag = s.tagName
	customResourceRecord.Value = `{"col1": "sls", "col2": "tag"}`
	return s.client.CreateResourceRecord(s.resourceName, customResourceRecord)
}

func (s *ResourceRecordTestSuite) TestClient_CreateResourceRecord() {
	err := s.createResourceRecord()
	s.Require().Nil(err)
	err = s.client.DeleteResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
}

func (s *ResourceRecordTestSuite) TestClient_UpdateResourceRecord() {
	err := s.createResourceRecord()
	s.Require().Nil(err)
	resourceRecord, err := s.client.GetResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
	resourceRecord.Value = `{"col1": "new sls", "col2": "new tag"}`
	err = s.client.UpdateResourceRecord(s.resourceName, resourceRecord)
	s.Require().Nil(err)
	resourceRecord, err = s.client.GetResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
	s.Require().Equal(`{"col1": "new sls", "col2": "new tag"}`, resourceRecord.Value, "update resourceRecord failed")
	err = s.client.DeleteResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
}

func (s *ResourceRecordTestSuite) TestClient_DeleteResourceRecord() {
	err := s.createResourceRecord()
	s.Require().Nil(err)
	_, err = s.client.GetResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
	err = s.client.DeleteResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
	_, err = s.client.GetResourceRecord(s.resourceName, s.recordId)
	s.Require().NotNil(err)
}

func (s *ResourceRecordTestSuite) TestClient_GetResourceRecord() {
	err := s.createResourceRecord()
	s.Require().Nil(err)
	getResourceRecord, err := s.client.GetResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
	s.Require().Equal(getResourceRecord.Id, s.recordId)

	err = s.client.DeleteResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
}

func (s *ResourceRecordTestSuite) TestClient_ListResourceRecord() {
	err := s.createResourceRecord()
	s.Require().Nil(err)
	resourceRecords, count, total, err := s.client.ListResourceRecord(s.resourceName, 0, 100)
	s.Require().Nil(err)
	if total != 1 || count != 1 {
		s.Require().Fail("list resourceRecord failed")
	}
	s.Require().Equal(1, len(resourceRecords), "there should be only one resourceRecord")
	resourceRecord := resourceRecords[0]
	s.Require().Equal(s.recordId, resourceRecord.Id, "list resourceRecord failed")
	err = s.client.DeleteResourceRecord(s.resourceName, s.recordId)
	s.Require().Nil(err)
}
