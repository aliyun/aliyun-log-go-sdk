package main

import (
	"fmt"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	config "github.com/aliyun/aliyun-log-go-sdk/example/config/util"
	"github.com/aliyun/aliyun-log-go-sdk/example/util"
)

func delimiter_example() {
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
	err = createDelimiterConfig(testConf, util.ProjectName, util.LogStoreName)
	if err != nil {
		fmt.Println("create config fail:", err)
		return
	}
	fmt.Println("create common regex logtail config sucessed")

	updateDelimiterConfig(testConf)
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
	fmt.Println("delete common regex logtail config sucessed")
	exist, err = config.CheckConfigExist(testConf)
	if err != nil {
		return
	}
	if exist {
		fmt.Println("config:" + testConf + " should not be exist")
		return
	}
	fmt.Println("log config sample end")

}

func createDelimiterConfig(configName string, projectName string, logstore string) (err error) {
	delimiterConfig := new(sls.DelimiterConfigInputDetail)
	sls.InitDelimiterConfigInputDetail(delimiterConfig)
	delimiterConfig.Quote = "\u0001"
	delimiterConfig.Key = []string{"1", "2", "3", "4", "5"}
	delimiterConfig.Separator = "\""
	// TimeKey and TimeFormat are optional, use system time as log time if not configed
	delimiterConfig.TimeKey = "1"
	delimiterConfig.TimeFormat = "xxxx"
	delimiterConfig.LogPath = "/var/log/log"
	delimiterConfig.FilePattern = "xxxx.log"
	outputDetail := sls.OutputDetail{
		ProjectName:  projectName,
		LogStoreName: logstore,
	}
	logConfig := &sls.LogConfig{
		Name:         configName,
		InputType:    "file",
		OutputType:   "LogService", // Now only supports LogService
		InputDetail:  delimiterConfig,
		OutputDetail: outputDetail,
	}
	err = util.Client.CreateConfig(projectName, logConfig)
	if err != nil {
		return err
	}
	return nil
}

func updateDelimiterConfig(configName string) {
	logtailConfig, _ := util.Client.GetConfig(util.ProjectName, configName)
	inputDetail, _ := sls.ConvertToDelimiterConfigInputDetail(logtailConfig.InputDetail)
	inputDetail.FilePattern = "*.log"
	err := util.Client.UpdateConfig(util.ProjectName, logtailConfig)
	if err != nil {
		panic(err)
	}
}
