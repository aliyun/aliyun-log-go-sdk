package main

import (
	"fmt"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	config "github.com/aliyun/aliyun-log-go-sdk/example/config/util"
	"github.com/aliyun/aliyun-log-go-sdk/example/util"
)

func config_example() {
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
	err = createPluginConfig(testConf, util.ProjectName, util.LogStoreName)
	if err != nil {
		fmt.Println("create config fail:", err)
		return
	}
	fmt.Println("create plugin logtail config sucessed")

	updatePluginConfig(testConf)
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
	fmt.Println("delete plugin logtail config sucessed")
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

func createPluginConfig(configName string, projectName string, logstore string) (err error) {
	pluginConfig := new(sls.PluginLogConfigInputDetail)
	sls.InitPluginLogConfigInputDetail(pluginConfig)
	dockerStdoutPlugin := sls.LogConfigPluginInput{}
	dockerStdoutPluginDetail := sls.CreateConfigPluginDockerStdout()
	dockerStdoutPluginDetail.IncludeEnv = map[string]string{
		"x":    "y",
		"dddd": "",
	}
	dockerStdoutPluginDetail.ExcludeEnv = map[string]string{
		"no_this_env": "",
	}
	dockerStdoutPlugin.Inputs = append(dockerStdoutPlugin.Inputs, sls.CreatePluginInputItem(sls.PluginInputTypeDockerStdout, dockerStdoutPluginDetail))

	pluginConfig.PluginDetail = dockerStdoutPlugin
	outputDetail := sls.OutputDetail{
		ProjectName:  projectName,
		LogStoreName: logstore,
	}
	logConfig := &sls.LogConfig{
		Name:         configName,
		InputType:    "plugin",
		OutputType:   "LogService", // Now only supports LogService
		InputDetail:  pluginConfig,
		OutputDetail: outputDetail,
	}
	err = util.Client.CreateConfig(projectName, logConfig)
	if err != nil {
		return err
	}
	return nil
}

func updatePluginConfig(configName string) {
	logtailConfig, _ := util.Client.GetConfig(util.ProjectName, configName)
	inputDetail, _ := sls.ConvertToPluginLogConfigInputDetail(logtailConfig.InputDetail)
	inputDetail.AdjustTimeZone = true
	err := util.Client.UpdateConfig(util.ProjectName, logtailConfig)
	if err != nil {
		panic(err)
	}
}
