package sls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestAlert(t *testing.T) {
	suite.Run(t, new(AlertTestSuite))
}

type AlertTestSuite struct {
	suite.Suite
	endpoint        string
	projectName     string
	logstoreName    string
	accessKeyID     string
	accessKeySecret string
	Project         *LogProject
	Logstore        *LogStore
	alertName       string
	dashboardName   string
	client          *Client
}

func (s *AlertTestSuite) SetupSuite() {
	s.endpoint = os.Getenv("LOG_TEST_ENDPOINT")
	s.projectName = fmt.Sprintf("test-go-alert-%d", time.Now().Unix())
	s.logstoreName = fmt.Sprintf("logstore-%d", time.Now().Unix())
	s.accessKeyID = os.Getenv("LOG_TEST_ACCESS_KEY_ID")
	s.accessKeySecret = os.Getenv("LOG_TEST_ACCESS_KEY_SECRET")
	slsProject, err := NewLogProject(s.projectName, s.endpoint, s.accessKeyID, s.accessKeySecret)
	s.Nil(err)
	s.NotNil(slsProject)
	s.Project = slsProject
	s.dashboardName = fmt.Sprintf("go-test-dashboard-%d", time.Now().Unix())
	s.alertName = fmt.Sprintf("go-test-alert-%d", time.Now().Unix())
	s.client = &Client{
		AccessKeyID:     s.accessKeyID,
		AccessKeySecret: s.accessKeySecret,
		Endpoint:        s.endpoint,
	}
	s.setUp()
}

func (s *AlertTestSuite) TearDownSuite() {
	err := s.client.DeleteProject(s.projectName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) createAlert() error {
	alerts, _, _, err := s.client.ListAlert(s.projectName, "", "", 0, 100)
	s.Require().Nil(err)
	for _, x := range alerts {
		err = s.client.DeleteAlert(s.projectName, x.Name)
		s.Require().Nil(err)
	}
	dashboard := Dashboard{
		DashboardName: s.dashboardName,
		DisplayName:   "test-dashboard",
		Description:   "test dashboard",
		ChartList:     []Chart{},
	}
	err = s.client.CreateDashboard(s.projectName, dashboard)
	if err != nil {
		slsErr := err.(*Error)
		if slsErr.Message != "specified dashboard already exists" {
			s.Require().Fail(slsErr.Message)
		}
	}
	alert := &Alert{
		Name:        s.alertName,
		Status:      "ENABLED",
		DisplayName: "AlertTest By GO SDK",
		Description: "Description for alert",
		Schedule: &Schedule{
			Type:     ScheduleTypeFixedRate,
			Interval: "1h",
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
			Dashboard:  s.dashboardName,
			MuteUntil:  time.Now().Unix() + 3600,
			Throttling: "5m",
			Condition:  "count > 0",
			NotificationList: []*Notification{
				{
					Type:      NotificationTypeEmail,
					Content:   "${alertName} triggered at ${firetime}",
					EmailList: []string{"test@abc.com"},
				},
				{
					Type:       NotificationTypeSMS,
					Content:    "${alertName} triggered at ${firetime}",
					MobileList: []string{"1234567891"},
				},
				{
					Type:       NotificationTypeWebhook,
					Method:     "OPTIONS",
					Content:    "${alertName} triggered at ${firetime}",
					Headers:    map[string]string{"content-type": "test", "name": "aliyun"},
					ServiceUri: "https://www.aliyun.com/",
				},
			},
			NotifyThreshold: 1,
		},
	}
	return s.client.CreateAlert(s.projectName, alert)
}

func (s *AlertTestSuite) createAlert2() error {
	alerts, _, _, err := s.client.ListAlert(s.projectName, "", "", 0, 100)
	s.Require().Nil(err)
	for _, x := range alerts {
		err = s.client.DeleteAlert(s.projectName, x.Name)
		s.Require().Nil(err)
	}
	alert := &Alert{
		Name:        s.alertName,
		Status:      "ENABLED",
		DisplayName: "AlertTest By GO SDK ",
		Description: "Description for alert by go sdk",
		Schedule: &Schedule{
			Type:     ScheduleTypeFixedRate,
			Interval: "1m",
		},
		Configuration: &AlertConfiguration{
			GroupConfiguration: GroupConfiguration{
				Type: GroupTypeNoGroup,
			},
			QueryList: []*AlertQuery{
				{
					Query:        "* | select count(1) as count",
					Start:        "-120s",
					End:          "now",
					TimeSpanType: "Custom",
					StoreType:    StoreTypeLog,
					Store:        "test-alert",
					Region:       "cn-hangzhou",
					Project:      s.projectName,
					PowerSqlMode: PowerSqlModeAuto,
				},
			},
			Dashboard:      s.dashboardName,
			MuteUntil:      time.Now().Unix(),
			Version:        "2.0",
			Type:           "default",
			Threshold:      1,
			NoDataFire:     true,
			NoDataSeverity: Medium,
			SendResolved:   true,
			Annotations: []*Tag{
				&Tag{
					Key:   "title",
					Value: "this is title",
				},
				&Tag{
					Key:   "desc",
					Value: "this is desc, count is ${count}",
				},
			},
			Labels: []*Tag{
				&Tag{
					Key:   "env",
					Value: "test",
				},
			},
			SeverityConfigurations: []*SeverityConfiguration{
				&SeverityConfiguration{
					Severity: Critical,
					EvalCondition: ConditionConfiguration{
						Condition: "count > 99",
					},
				},
				&SeverityConfiguration{
					Severity: High,
					EvalCondition: ConditionConfiguration{
						Condition: "count > 80",
					},
				},
				&SeverityConfiguration{
					Severity: Medium,
					EvalCondition: ConditionConfiguration{
						Condition: "count > 20",
					},
				},
				&SeverityConfiguration{
					Severity:      Report,
					EvalCondition: ConditionConfiguration{},
				},
			},
			PolicyConfiguration: PolicyConfiguration{
				AlertPolicyId:  "sls.builtin.dynamic",
				ActionPolicyId: "huolang.test",
				RepeatInterval: "5m",
			},
			AutoAnnotation: true,
			SinkEventStore: &SinkEventStoreConfiguration{
				Enabled:    true,
				RoleArn:    "acs:ram::${uid}:role/aliyunlogetlrole",
				Project:    s.projectName,
				Endpoint:   s.endpoint,
				EventStore: "alert-eventstore",
			},
			SinkAlerthub: &SinkAlerthubConfiguration{
				Enabled: true,
			},
			SinkCms: &SinkCmsConfiguration{
				Enabled: true,
			},
		},
	}
	return s.client.CreateAlert(s.projectName, alert)
}

func (s *AlertTestSuite) TestClient_CreateAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_CreateAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_UpdateAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	alert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert.DisplayName = "new display name"
	alert.CreateTime = 0
	alert.LastModifiedTime = 0
	err = s.client.UpdateAlert(s.projectName, alert)
	s.Require().Nil(err)
	alert, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("new display name", alert.DisplayName, "update alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_UpdateAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	alert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert.DisplayName = "new display name"
	alert.CreateTime = 0
	alert.LastModifiedTime = 0
	err = s.client.UpdateAlert(s.projectName, alert)
	s.Require().Nil(err)
	alert, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("new display name", alert.DisplayName, "update alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_DeleteAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	_, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	_, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().NotNil(err)
}

func (s *AlertTestSuite) TestClient_DeleteAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	_, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	_, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().NotNil(err)
}

func (s *AlertTestSuite) TestClient_DisableAndEnableAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	err = s.client.DisableAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("DISABLED", alert.Status, "disable alert failed")
	err = s.client.EnableAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("ENABLED", alert.Status, "enable alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_DisableAndEnableAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	err = s.client.DisableAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("DISABLED", alert.Status, "disable alert failed")
	err = s.client.EnableAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	alert, err = s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal("ENABLED", alert.Status, "enable alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_GetAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	getAlert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal(getAlert.Name, s.alertName)
	s.Require().Equal(len(getAlert.Configuration.NotificationList), 3)
	for _, v := range getAlert.Configuration.NotificationList {
		if v.Type == NotificationTypeWebhook {
			s.Require().Equal(v.Method, "OPTIONS")
			s.Require().Equal(v.Headers, map[string]string{"content-type": "test", "name": "aliyun"})
		}
	}

	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_GetAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	getAlert, err := s.client.GetAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
	s.Require().Equal(getAlert.Name, s.alertName)
	s.Require().Equal(getAlert.Configuration.GroupConfiguration.Type, GroupTypeNoGroup)
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_ListAlert() {
	err := s.createAlert()
	s.Require().Nil(err)
	alerts, total, count, err := s.client.ListAlert(s.projectName, "", "", 0, 100)
	s.Require().Nil(err)
	if total != 1 || count != 1 {
		s.Require().Fail("list alert failed")
	}
	s.Require().Equal(1, len(alerts), "there should be only one alert")
	alert := alerts[0]
	s.Require().Equal(s.alertName, alert.Name, "list alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) TestClient_ListAlert2() {
	err := s.createAlert2()
	s.Require().Nil(err)
	alerts, total, count, err := s.client.ListAlert(s.projectName, "", "", 0, 100)
	s.Require().Nil(err)
	if total != 1 || count != 1 {
		s.Require().Fail("list alert failed")
	}
	s.Require().Equal(1, len(alerts), "there should be only one alert")
	alert := alerts[0]
	s.Require().Equal(s.alertName, alert.Name, "list alert failed")
	err = s.client.DeleteAlert(s.projectName, s.alertName)
	s.Require().Nil(err)
}

func (s *AlertTestSuite) setUp() {
	_, ce := s.client.CreateProject(s.projectName, "test alert")
	s.Require().Nil(ce)
	time.Sleep(time.Second * 60)
	cle := s.client.CreateLogStore(s.projectName, s.logstoreName, 3, 2, true, 4)
	s.Require().Nil(cle)
	cie := s.client.CreateIndex(s.projectName, s.logstoreName, Index{
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
