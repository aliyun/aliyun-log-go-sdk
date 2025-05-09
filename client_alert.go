package sls

import (
	"encoding/json"
	"strconv"
)

// SavedSearch ...
type SavedSearch struct {
	SavedSearchName string `json:"savedsearchName"`
	SearchQuery     string `json:"searchQuery"`
	Logstore        string `json:"logstore"`
	Topic           string `json:"topic"`
	DisplayName     string `json:"displayName"`
}

type ResponseSavedSearchItem struct {
	SavedSearchName string `json:"savedsearchName"`
	DisplayName     string `json:"displayName"`
}

const (
	NotificationTypeSMS           = "SMS"
	NotificationTypeWebhook       = "Webhook"
	NotificationTypeDingTalk      = "DingTalk"
	NotificationTypeEmail         = "Email"
	NotificationTypeMessageCenter = "MessageCenter"
)

const (
	CountConditionKey = "__count__"
)

type Severity int

const (
	Report   Severity = 2
	Low      Severity = 4
	Medium   Severity = 6
	High     Severity = 8
	Critical Severity = 10
)

// power sql
type PowerSqlMode string

const (
	PowerSqlModeAuto    PowerSqlMode = "auto"
	PowerSqlModeEnable  PowerSqlMode = "enable"
	PowerSqlModeDisable PowerSqlMode = "disable"
)

const (
	JoinTypeCross        = "cross_join"
	JoinTypeInner        = "inner_join"
	JoinTypeLeft         = "left_join"
	JoinTypeRight        = "right_join"
	JoinTypeFull         = "full_join"
	JoinTypeLeftExclude  = "left_exclude"
	JoinTypeRightExclude = "right_exclude"
	JoinTypeConcat       = "concat"
	JoinTypeNo           = "no_join"
)

const (
	GroupTypeNoGroup    = "no_group"
	GroupTypeLabelsAuto = "labels_auto"
	GroupTypeCustom     = "custom"
)

const (
	ScheduleTypeFixedRate = "FixedRate"
	ScheduleTypeHourly    = "Hourly"
	ScheduleTypeDaily     = "Daily"
	ScheduleTypeWeekly    = "Weekly"
	ScheduleTypeCron      = "Cron"
	ScheduleTypeDayRun    = "DryRun"
	ScheduleTypeResident  = "Resident"
)

const (
	StoreTypeLog    = "log"
	StoreTypeMetric = "metric"
	StoreTypeMeta   = "meta"
)

// SeverityConfiguration severity config by group
type SeverityConfiguration struct {
	Severity      Severity               `json:"severity"`
	EvalCondition ConditionConfiguration `json:"evalCondition"`
}

type ConditionConfiguration struct {
	Condition      string `json:"condition"`
	CountCondition string `json:"countCondition"`
}

type JoinConfiguration struct {
	Type      string `json:"type"`
	Condition string `json:"condition"`
}

type GroupConfiguration struct {
	Type   string   `json:"type"`
	Fields []string `json:"fields"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Token struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Hide        bool   `json:"hide"`
}

type TemplateConfiguration struct {
	Id          string            `json:"id"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	Lang        string            `json:"lang"`
	Tokens      map[string]string `json:"tokens"`
	Annotations map[string]string `json:"annotations"`
}

type PolicyConfiguration struct {
	UseDefault     bool   `json:"useDefault"`
	RepeatInterval string `json:"repeatInterval"`
	AlertPolicyId  string `json:"alertPolicyId"`
	ActionPolicyId string `json:"actionPolicyId"`
}

type SinkEventStoreConfiguration struct {
	Enabled    bool   `json:"enabled"`
	Endpoint   string `json:"endpoint"`
	Project    string `json:"project"`
	EventStore string `json:"eventStore"`
	RoleArn    string `json:"roleArn"`
}

type SinkCmsConfiguration struct {
	Enabled bool `json:"enabled"`
}

type SinkAlerthubConfiguration struct {
	Enabled bool `json:"enabled"`
}

type Alert struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	// Deprecated: use `alert.IsEnabled()` to get the status, use api EnableAlert and DisableAlert to enable/disable the alert
	State            string              `json:"state,omitempty"`
	Status           string              `json:"status,omitempty"`
	Configuration    *AlertConfiguration `json:"configuration"`
	Schedule         *Schedule           `json:"schedule"`
	CreateTime       int64               `json:"createTime,omitempty"`
	LastModifiedTime int64               `json:"lastModifiedTime,omitempty"`
}

func (alert *Alert) IsEnabled() bool {
	return alert.Status != "DISABLED"
}

func (alert *Alert) MarshalJSON() ([]byte, error) {
	body := map[string]interface{}{
		"name":          alert.Name,
		"displayName":   alert.DisplayName,
		"description":   alert.Description,
		"configuration": alert.Configuration,
		"schedule":      alert.Schedule,
		"type":          "Alert",
	}
	if alert.State != "" {
		body["state"] = alert.State
	}
	if alert.Status != "" {
		body["status"] = alert.Status
	}
	return json.Marshal(body)
}

type AlertQuery struct {
	ChartTitle   string `json:"chartTitle"`
	LogStore     string `json:"logStore"`
	Query        string `json:"query"`
	TimeSpanType string `json:"timeSpanType"`
	Start        string `json:"start"`
	End          string `json:"end"`

	StoreType    string       `json:"storeType"`
	Project      string       `json:"project"`
	Store        string       `json:"store"`
	Region       string       `json:"region"`
	RoleArn      string       `json:"roleArn"`
	DashboardId  string       `json:"dashboardId"`
	PowerSqlMode PowerSqlMode `json:"powerSqlMode"`
}

type Notification struct {
	Type       string            `json:"type"`
	Content    string            `json:"content"`
	EmailList  []string          `json:"emailList,omitempty"`
	Method     string            `json:"method,omitempty"`
	MobileList []string          `json:"mobileList,omitempty"`
	ServiceUri string            `json:"serviceUri,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

type Schedule struct {
	Type           string `json:"type"`
	Interval       string `json:"interval"`
	CronExpression string `json:"cronExpression"`
	Delay          int32  `json:"delay"`
	DayOfWeek      int32  `json:"dayOfWeek"`
	Hour           int32  `json:"hour"`
	RunImmediately bool   `json:"runImmediately"`
	TimeZone       string `json:"timeZone,omitempty"`
}

type AlertConfiguration struct {
	Condition        string          `json:"condition"`
	MuteUntil        int64           `json:"muteUntil,omitempty"`
	NotificationList []*Notification `json:"notificationList"`
	NotifyThreshold  int32           `json:"notifyThreshold"`
	Throttling       string          `json:"throttling"`

	Version               string                 `json:"version"`
	Type                  string                 `json:"type"`
	TemplateConfiguration *TemplateConfiguration `json:"templateConfiguration"`

	Dashboard              string                   `json:"dashboard"`
	Threshold              int                      `json:"threshold"`
	NoDataFire             bool                     `json:"noDataFire"`
	NoDataSeverity         Severity                 `json:"noDataSeverity"`
	SendResolved           bool                     `json:"sendResolved"`
	QueryList              []*AlertQuery            `json:"queryList"`
	Annotations            []*Tag                   `json:"annotations"`
	Labels                 []*Tag                   `json:"labels"`
	SeverityConfigurations []*SeverityConfiguration `json:"severityConfigurations"`

	JoinConfigurations []*JoinConfiguration `json:"joinConfigurations"`
	GroupConfiguration GroupConfiguration   `json:"groupConfiguration"`

	PolicyConfiguration PolicyConfiguration          `json:"policyConfiguration"`
	AutoAnnotation      bool                         `json:"autoAnnotation"`
	SinkEventStore      *SinkEventStoreConfiguration `json:"sinkEventStore"`
	SinkCms             *SinkCmsConfiguration        `json:"sinkCms"`
	SinkAlerthub        *SinkAlerthubConfiguration   `json:"sinkAlerthub"`

	Tags []string `json:"tags,omitempty"`
}

func (c *Client) CreateSavedSearch(project string, savedSearch *SavedSearch) error {
	return c.doRequest(project, "POST", "/savedsearches", nil, nil, savedSearch, nil)
}

func (c *Client) UpdateSavedSearch(project string, savedSearch *SavedSearch) error {
	path := "/savedsearches/" + savedSearch.SavedSearchName
	return c.doRequest(project, "PUT", path, nil, nil, savedSearch, nil)
}

func (c *Client) DeleteSavedSearch(project string, savedSearchName string) error {
	path := "/savedsearches/" + savedSearchName
	return c.doRequest(project, "DELETE", path, nil, nil, nil, nil)
}

func (c *Client) GetSavedSearch(project string, savedSearchName string) (*SavedSearch, error) {
	path := "/savedsearches/" + savedSearchName
	var savedSearch SavedSearch
	if err := c.doRequest(project, "GET", path, nil, nil, nil, &savedSearch); err != nil {
		return nil, err
	}
	return &savedSearch, nil
}

func (c *Client) ListSavedSearch(project string, savedSearchName string, offset, size int) (savedSearches []string, total int, count int, err error) {
	queryParams := map[string]string{
		"offset":          strconv.Itoa(offset),
		"size":            strconv.Itoa(size),
		"savedsearchName": savedSearchName,
	}
	type ListSavedSearch struct {
		Total         int      `json:"total"`
		Count         int      `json:"count"`
		Savedsearches []string `json:"savedsearches"`
	}
	var listSavedSearch ListSavedSearch
	if err = c.doRequest(project, "GET", "/savedsearches", queryParams, nil, nil, &listSavedSearch); err != nil {
		return nil, 0, 0, err
	}
	return listSavedSearch.Savedsearches, listSavedSearch.Total, listSavedSearch.Count, nil
}

func (c *Client) ListSavedSearchV2(project string, savedSearchName string, offset, size int) (savedSearches []string, savedsearchItems []ResponseSavedSearchItem, total int, count int, err error) {
	queryParams := map[string]string{
		"offset":          strconv.Itoa(offset),
		"size":            strconv.Itoa(size),
		"savedsearchName": savedSearchName,
	}
	type ListSavedSearch struct {
		Total            int                       `json:"total"`
		Count            int                       `json:"count"`
		Savedsearches    []string                  `json:"savedsearches"`
		SavedsearchItems []ResponseSavedSearchItem `json:"savedsearchItems"`
	}
	var listSavedSearch ListSavedSearch
	if err = c.doRequest(project, "GET", "/savedsearches", queryParams, nil, nil, &listSavedSearch); err != nil {
		return nil, nil, 0, 0, err
	}
	return listSavedSearch.Savedsearches, listSavedSearch.SavedsearchItems, listSavedSearch.Total, listSavedSearch.Count, nil
}

func (c *Client) CreateAlert(project string, alert *Alert) error {
	return c.doRequest(project, "POST", "/jobs", nil, nil, alert, nil)
}

func (c *Client) CreateAlertString(project string, alert string) error {
	return c.doRequest(project, "POST", "/jobs", nil, map[string]string{
		HTTPHeaderContentType: "application/json",
	}, []byte(alert), nil)
}

func (c *Client) UpdateAlert(project string, alert *Alert) error {
	path := "/jobs/" + alert.Name
	return c.doRequest(project, "PUT", path, nil, nil, alert, nil)
}

func (c *Client) UpdateAlertString(project string, alertName, alert string) error {
	path := "/jobs/" + alertName
	return c.doRequest(project, "PUT", path, nil, map[string]string{
		HTTPHeaderContentType: "application/json",
	}, []byte(alert), nil)
}

func (c *Client) DeleteAlert(project string, alertName string) error {
	path := "/jobs/" + alertName
	return c.doRequest(project, "DELETE", path, nil, nil, nil, nil)
}

func (c *Client) DisableAlert(project string, alertName string) error {
	path := "/jobs/" + alertName
	return c.doRequest(project, "PUT", path, map[string]string{
		"action": "disable",
	}, nil, nil, nil)
}

func (c *Client) EnableAlert(project string, alertName string) error {
	path := "/jobs/" + alertName
	return c.doRequest(project, "PUT", path, map[string]string{
		"action": "enable",
	}, nil, nil, nil)
}

func (c *Client) GetAlert(project string, alertName string) (*Alert, error) {
	path := "/jobs/" + alertName
	var alert Alert
	if err := c.doRequest(project, "GET", path, nil, nil, nil, &alert); err != nil {
		return nil, err
	}
	return &alert, nil
}

func (c *Client) GetAlertString(project string, alertName string) (string, error) {
	path := "/jobs/" + alertName
	body, err := c.doRequestRaw(project, "GET", path, nil, nil, nil)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) ListAlert(project, alertName, dashboard string, offset, size int) (alerts []*Alert, total int, count int, err error) {
	queryParams := map[string]string{
		"offset":  strconv.Itoa(offset),
		"size":    strconv.Itoa(size),
		"jobName": alertName,
		"jobType": "Alert",
	}
	if dashboard != "" {
		queryParams["resourceProvider"] = dashboard
	}
	type AlertList struct {
		Total   int      `json:"total"`
		Count   int      `json:"count"`
		Results []*Alert `json:"results"`
	}
	var listAlert AlertList
	if err = c.doRequest(project, "GET", "/jobs", queryParams, nil, nil, &listAlert); err != nil {
		return nil, 0, 0, err
	}
	return listAlert.Results, listAlert.Total, listAlert.Count, nil
}

func (c *Client) PublishAlertEvent(project string, alertResult []byte) error {
	return c.doRequest(project, "POST", "/event/alerthub", map[string]string{
		"type": "raw",
	}, nil, alertResult, nil)
}
