package sls

import (
	"math/rand"
	"strconv"

	"github.com/Netflix/go-env"
	"github.com/stretchr/testify/suite"
)

// for internal use
type functionTestSuiteBase struct {
	suite.Suite
	Client           ClientInterface
	Endpoint         string `env:"LOG_TEST_ENDPOINT"`
	TestProjectName  string `env:"LOG_TEST_PROJECT"`
	TestLogstoreName string `env:"LOG_TEST_LOGSTORE"`
	AccessKeyID      string `env:"LOG_TEST_ACCESS_KEY_ID"`
	AccessKeySecret  string `env:"LOG_TEST_ACCESS_KEY_SECRET"`
}

func (b *functionTestSuiteBase) init() {
	_, err := env.UnmarshalFromEnviron(b)
	b.Require().NoError(err)
	b.Client = CreateNormalInterface(b.Endpoint, b.AccessKeyID, b.AccessKeySecret, "")
}

func makeTestProjectName() string {
	return "sls-sdk-testp-go-" + strconv.Itoa(rand.Intn(10000))
}

func makeTestLogStoreName() string {
	return "test-" + strconv.Itoa(rand.Intn(10000))
}

func (b *functionTestSuiteBase) createProject() string {
	projectName := makeTestProjectName()
	_, err := b.Client.CreateProject(projectName, "test project")
	b.Require().NoError(err)
	return projectName
}

func (b *functionTestSuiteBase) createProjectAndLogStore() (string, string) {
	projectName := b.createProject()
	logstoreName := makeTestLogStoreName()
	err := b.Client.CreateLogStore(projectName, logstoreName, 1, 2, false, 64)
	b.Require().NoError(err)
	return projectName, logstoreName
}

func (b *functionTestSuiteBase) cleanUpProject(projectName string) {
	_ = b.Client.DeleteProject(projectName)
}

func (b *functionTestSuiteBase) getClient() ClientInterface {
	return b.Client
}
