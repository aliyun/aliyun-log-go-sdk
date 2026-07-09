package sls

import (
	"encoding/json"
	"testing"
)

func TestDashboardJSONFields(t *testing.T) {
	raw := []byte(`{
		"dashboardName": "dashboard-test",
		"displayName": "display-test",
		"description": "",
		"attribute": {
			"update": "1772461212344",
			"remark": "modify chart",
			"type": "grid",
			"version": "39986382"
		},
		"charts": [
			{
				"title": "chart-test",
				"type": "aggpro",
				"action": {
					"open": {
						"type": "link"
					}
				},
				"search": {
					"query": "@",
					"start": "-900s",
					"topic": "",
					"end": "now",
					"logstore": "sls_op_log",
					"dataSourceType": "current",
					"chartQueries": [
						{
							"datasource": "logstore",
							"displayName": "current query",
							"query": "*",
							"name": "A",
							"tokenQuery": "*",
							"project": "project-test",
							"logstore": "logstore-test"
						},
						{
							"datasource": "metricstore",
							"query": "sum(rate(metric[1m]))",
							"limit": 10000,
							"name": "B",
							"tokenQuery": "sum(rate(metric[1m]))",
							"legendFormat": "worker",
							"project": "project-test",
							"logstore": "metricstore-test"
						}
					]
				},
				"display": {
					"xPos": 1,
					"yPos": 2,
					"width": 8,
					"height": 6,
					"displayName": ""
				}
			}
		]
	}`)

	var dashboard Dashboard
	if err := json.Unmarshal(raw, &dashboard); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got := dashboard.Attribute["update"]; got != "1772461212344" {
		t.Fatalf("Attribute[update] = %q, want %q", got, "1772461212344")
	}

	chart := dashboard.ChartList[0]
	if _, ok := chart.Action["open"]; !ok {
		t.Fatalf("Action[open] is missing")
	}

	if got := chart.Search.DataSourceType; got != "current" {
		t.Fatalf("DataSourceType = %q, want %q", got, "current")
	}

	if got := len(chart.Search.ChartQueries); got != 2 {
		t.Fatalf("len(ChartQueries) = %d, want %d", got, 2)
	}

	if got := chart.Search.ChartQueries[0].DisplayName; got != "current query" {
		t.Fatalf("ChartQueries[0].DisplayName = %q, want %q", got, "current query")
	}

	if got := chart.Search.ChartQueries[1].Limit; got != 10000 {
		t.Fatalf("ChartQueries[1].Limit = %d, want %d", got, 10000)
	}

	buf, err := json.Marshal(dashboard)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var roundTrip map[string]any
	if err := json.Unmarshal(buf, &roundTrip); err != nil {
		t.Fatalf("round-trip json.Unmarshal() error = %v", err)
	}

	if _, ok := roundTrip["attribute"]; !ok {
		t.Fatalf("marshaled dashboard is missing attribute")
	}
}
