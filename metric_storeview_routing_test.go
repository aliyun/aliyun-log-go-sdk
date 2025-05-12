package sls

import (
	"fmt"
	"testing"
)

func Test_StoreView_Routing_Checker(t *testing.T) {
	storeViewRoutingConfig :=
		`[
			{
				"metric_names": [ ".*api.*", ".*bucket" ],
				"project_stores":
				[
					{
						"project": "test_project2",
						"metricstore": "prometheus2"
					},
					{
						"project": ".*project5",
						"metricstore": ".*"
					},
					{
						"project": "test_project1|test_project2|test_project3",
						"metricstore": "prometheus1|store02|store03"
					}
				]
			},
			{
				"metric_names": [ "influx_.*" ],
				"project_stores":
				[
					{
						"project": "test_project3",
						"metricstore": "influx_metric1"
					},
					{
						"project": ".*mock_project.*",
						"metricstore": ".*influx_metric.*"
					},
					{
						"project": "test_project4",
						"metricstore": ".*influx_metric2.*"
					}
				]
			},
			{
				"metric_names": [ "none_exist_metric" ],
				"project_stores":
				[
					{
						"project": ".*",
						"metricstore": ".*"
					}
				]
			}
		]`

	routingChecker, err := NewStoreViewRoutingChecker([]byte(storeViewRoutingConfig))
	if err != nil {
		panic(err)
	}

	projectStores := []ProjectStore{
		{
			ProjectName: "test_project1",
			MetricStore: "prometheus1",
		}, {
			ProjectName: "test_project2",
			MetricStore: "prometheus2",
		},
		{
			ProjectName: "test_project3",
			MetricStore: "influx_metric1",
		},
		{
			ProjectName: "test_project4",
			MetricStore: "influx_metric2",
		},
		{
			ProjectName: "test_project5",
			MetricStore: "k8s_metric1",
		},
		{
			ProjectName: "test_project6",
			MetricStore: "k8s_metric2",
		},
	}

	queries := []string{
		"api_metric or mock_metric01",
		"influx_metric1 / api_duration_bucket",
	}

	for _, query := range queries {
		fmt.Println("\n================== ", query, " ==================")
		dstProjects, err := routingChecker.CheckPromQlQuery(query, projectStores)
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, dstProject := range dstProjects {
			fmt.Println("####")
			fmt.Println("-- metric: ", dstProject.MetricName)
			fmt.Println("-- stores: ", dstProject.ProjectStores)
		}
	}

}
