package sls

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type SavedSerarchTestSuite struct {
	functionTestSuiteBase
	projectName  string
	logstoreName string
}

func TestSavedSearchFunctionTest(t *testing.T) {
	suite.Run(t, new(SavedSerarchTestSuite))
}

func (s *SavedSerarchTestSuite) SetupSuite() {
	s.functionTestSuiteBase.init()
	s.projectName, s.logstoreName = s.createProjectAndLogStore()
}

func (s *SavedSerarchTestSuite) TearDownSuite() {
	s.cleanUpProject(s.projectName)
}

func (s *SavedSerarchTestSuite) TestSavedSearch() {
	c := s.getClient().(*Client)
	name := "test-savedsearch"
	err := c.CreateSavedSearch(s.projectName, &SavedSearch{
		SavedSearchName: name,
		SearchQuery:     "*",
		DisplayName:     "test-savedsearch-display",
		Logstore:        s.logstoreName,
	})
	s.Require().NoError(err)

	savedSearch, err := c.GetSavedSearch(s.projectName, name)
	s.Require().NoError(err)
	s.Equal(name, savedSearch.SavedSearchName)
	s.Equal("test-savedsearch-display", savedSearch.DisplayName)
	s.Equal(s.logstoreName, savedSearch.Logstore)
	s.Equal("*", savedSearch.SearchQuery)

	{
		searches, total, line, err := c.ListSavedSearch(s.projectName, "", 0, 10)
		s.Require().NoError(err)
		s.Equal(1, line)
		s.Equal(1, total)
		s.Equal(1, len(searches))
		s.Equal(name, searches[0])
	}
	{
		searches, total, line, err := c.ListSavedSearch(s.projectName, name, 0, 10)
		s.Require().NoError(err)
		s.Equal(1, line)
		s.Equal(1, total)
		s.Equal(1, len(searches))
		s.Equal(name, searches[0])
	}
	{
		searches, total, line, err := c.ListSavedSearch(s.projectName, name+"something", 0, 10)
		s.Require().NoError(err)
		s.Equal(0, line)
		s.Equal(0, total)
		s.Equal(0, len(searches))
	}
	{
		searches, items, total, line, err := c.ListSavedSearchV2(s.projectName, name, 0, 10)
		s.Require().NoError(err)
		s.Equal(1, line)
		s.Equal(1, total)
		s.Equal(1, len(searches))
		s.Equal(1, len(items))
		s.Equal(name, searches[0])
	}
	err = c.UpdateSavedSearch(s.projectName, &SavedSearch{
		SavedSearchName: name,
		SearchQuery:     "* | select count(*) as cnt",
		DisplayName:     "test-savedsearch-display",
		Logstore:        s.logstoreName,
	})
	s.Require().NoError(err)
	savedSearch, err = c.GetSavedSearch(s.projectName, name)
	s.Require().NoError(err)
	s.Equal("* | select count(*) as cnt", savedSearch.SearchQuery)

	err = c.DeleteSavedSearch(s.projectName, name)
	s.Require().NoError(err)

	_, err = c.GetSavedSearch(s.projectName, name)
	s.Require().Error(err)
}

type AlertFuncTestSuite struct {
	functionTestSuiteBase
	projectName  string
	logstoreName string
}

func TestAlertFunctionTest(t *testing.T) {
	suite.Run(t, new(AlertFuncTestSuite))
}

func (s *AlertFuncTestSuite) SetupSuite() {
	s.functionTestSuiteBase.init()
	s.projectName, s.logstoreName = s.createProjectAndLogStore()
}

func (s *AlertFuncTestSuite) TearDownSuite() {
	s.cleanUpProject(s.projectName)
}

func (s *AlertFuncTestSuite) TestAlertFunc() {
	c := s.getClient().(*Client)
	name := "test-alert"
	alert := &Alert{
		Name:        name,
		DisplayName: name,
		Description: "test",
		Schedule: &Schedule{
			Type:           ScheduleTypeHourly,
			RunImmediately: false,
		},
		Configuration: &AlertConfiguration{
			QueryList: []*AlertQuery{
				{
					ChartTitle:   "chart-abc",
					Query:        "* | select count(1) as count",
					Start:        "-120s",
					End:          "now",
					TimeSpanType: "Custom",
					LogStore:     "test-logstore",
				},
			},
			Condition: "count > 0",
			Dashboard: "test-dashboard",
			NotificationList: []*Notification{
				{
					Type:      NotificationTypeEmail,
					Content:   "${alertName} triggered at ${firetime}",
					EmailList: []string{"test@abc.com"},
				},
			},
		},
	}
	err := c.CreateAlert(s.projectName, alert)
	s.Require().NoError(err)

	alertResp, err := c.GetAlert(s.projectName, name)
	s.Require().NoError(err)
	s.Equal(alert.Description, alertResp.Description)
	s.Equal(alert.Schedule.Type, alertResp.Schedule.Type)

	s.True(alertResp.IsEnabled())

	alertString, err := c.GetAlertString(s.projectName, name)
	s.Require().NoError(err)
	s.Greater(len(alertString), 0)

	err = c.UpdateAlertString(s.projectName, name, alertString)
	s.Require().NoError(err)

	listResp, total, count, err := c.ListAlert(s.projectName, name, "", 0, 10)
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(1, count)
	s.Equal(1, len(listResp))
	s.Equal(name, listResp[0].Name)

	err = c.DisableAlert(s.projectName, name)
	s.Require().NoError(err)

	{
		getResp, err := c.GetAlert(s.projectName, name)
		s.Require().NoError(err)
		s.False(getResp.IsEnabled())
	}

	err = c.EnableAlert(s.projectName, name)
	s.Require().NoError(err)
	{
		getResp, err := c.GetAlert(s.projectName, name)
		s.Require().NoError(err)
		s.True(getResp.IsEnabled())
	}

	err = c.DeleteAlert(s.projectName, name)
	s.Require().NoError(err)
}
