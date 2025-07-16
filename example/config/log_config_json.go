package main

import (
	"fmt"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	config "github.com/aliyun/aliyun-log-go-sdk/example/config/util"
	"github.com/aliyun/aliyun-log-go-sdk/example/util"
)

func config_json_example() {
	// log config sample
	testConf := "test-conf"
	exist, err := config.CheckConfigExist(testConf)
	fmt.Println(exist)
	if err != nil {
		fmt.Println("check conf exist fail:", err)
		return
	}
	if exist {
		config.DeleteConfig(testConf)
	}
	err = createJsonConfig(testConf, util.ProjectName, util.LogStoreName)
	if err != nil {
		fmt.Println("create config fail:", err)
		return
	}
	fmt.Println("create json logtail config sucessed")

	updateJsonConfig(testConf)
	config.GetConfig(testConf)

	exist, err = config.CheckConfigExist(testConf)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !exist {
		fmt.Println("config:" + testConf + " should be exist")
		return
	}

	config.DeleteConfig(testConf)
	fmt.Println("delete json logtail config sucessed")
	exist, err = config.CheckConfigExist(testConf)
	if err != nil {
		fmt.Println(err)
		return
	}
	if exist {
		fmt.Println("config:" + testConf + " should not be exist")
		return
	}
	fmt.Println("log config sample end")

}

func createJsonConfig(configName string, projectName string, logstore string) (err error) {
	jsonConfig := new(sls.JSONConfigInputDetail)
	sls.InitJSONConfigInputDetail(jsonConfig)
	// TimeKey and TimeFormat are optional, use system time as log time if not configed
	jsonConfig.TimeKey = "key_time"
	jsonConfig.TimeFormat = "%Y/%m/%d %H:%M:%S"
	jsonConfig.LogPath = "/cloud/log/"
	jsonConfig.FilePattern = "access.log*"
	outputDetail := sls.OutputDetail{
		ProjectName:  projectName,
		LogStoreName: logstore,
	}
	logConfig := &sls.LogConfig{
		Name:         configName,
		InputType:    "file",
		OutputType:   "LogService", // Now only supports LogService
		InputDetail:  jsonConfig,
		OutputDetail: outputDetail,
	}
	err = util.Client.CreateConfig(projectName, logConfig)
	if err != nil {
		return err
	}
	return nil
}

func updateJsonConfig(configName string) {
	logtailConfig, _ := util.Client.GetConfig(util.ProjectName, configName)
	inputDetail, _ := sls.ConvertToJSONConfigInputDetail(logtailConfig.InputDetail)
	inputDetail.FilePattern = "*.log"
	err := util.Client.UpdateConfig(util.ProjectName, logtailConfig)
	if err != nil {
		panic(err)
	}
}
