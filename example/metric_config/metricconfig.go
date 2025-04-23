package main

import (
	"encoding/json"
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/example/util"
)

func main() {
	config := "{\n  \"parallel_config\": {\n    \"enable\": true,\n    \"mode\": \"static\",\n    \"parallel_count_per_host\": 2,\n    \"time_piece_count\": 0,\n    \"time_piece_interval\": 21600,\n    \"total_parallel_count\": 8\n  },\n  \"query_cache_config\": {\n    \"enable\": true\n  }\n}"
	var conf sls.MetricsConfig
	json.Unmarshal([]byte(config), &conf)
	err := util.Client.CreateMetricConfig("test_project", "test_metric", &conf)
	if err != nil {
		panic(err)
	}

	Config, err := util.Client.GetMetricConfig("test_project", "test_metric")
	if err != nil {
		panic(err)
	}
	res, _ := json.Marshal(&Config)
	fmt.Println(string(res))

	config01 := "{\n  \"parallel_config\": {\n    \"enable\": true,\n    \"mode\": \"auto\",\n    \"parallel_count_per_host\": 2,\n    \"time_piece_count\": 6,\n    \"time_piece_interval\": 21600,\n    \"total_parallel_count\": 6\n  },\n  \"query_cache_config\": {\n    \"enable\": true\n  }\n}"
	json.Unmarshal([]byte(config01), &conf)
	err = util.Client.UpdateMetricConfig("test_project", "test_metric", &conf)
	if err != nil {
		panic(err)
	}

	Config, err = util.Client.GetMetricConfig("test_project", "test_metric")
	if err != nil {
		panic(err)
	}
	res, _ = json.Marshal(&Config)
	fmt.Println("--------------------")
	fmt.Println(string(res))

	err = util.Client.DeleteMetricConfig("test_project", "test_metric")
	if err != nil {
		panic(err)
	}
}
