package main

import (
	"encoding/json"
	"fmt"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
)

const (
	endpoint        = "your endpoint" // https://help.aliyun.com/document_detail/29008.html
	accessKeyId     = "your akId"
	accessKeySecret = "your akSecret"
	securityToken   = ""
	projectName     = "your project name"
	logStoreName    = "your logstore name"
	etlJobName      = "your etl job name"
	etlScript       = "your etl script"
)

func main() {
	// create the client with ak and endpoint
	client := sls.CreateNormalInterface(endpoint, accessKeyId, accessKeySecret, securityToken)

	// create the ETL Job
	if err := client.CreateETL(projectName, getETLJob(etlJobName, etlScript)); err != nil {
		fmt.Println(err)
	}

	// get the ETL job
	if etlJob, err := client.GetETL(projectName, etlJobName); err != nil {
		fmt.Println(err)
	} else {
		detail, _ := json.Marshal(etlJob)
		fmt.Println(string(detail))

		etlJob.Configuration.Script = "e_set(\"k\", \"v\")"
		// update the ETL Job
		if err := client.UpdateETL(projectName, *etlJob); err != nil {
			fmt.Println(err)
		}

		// update and restart the ETL Job
		if err := client.RestartETL(projectName, *etlJob); err != nil {
			fmt.Println(err)
		}
	}

	// list the ETL jobs under the project
	if etlJobs, err := client.ListETL(projectName, 0, 10); err != nil {
		fmt.Println(err)
	} else {
		detail, _ := json.Marshal(etlJobs.Results)
		fmt.Println(string(detail))
		fmt.Println(etlJobs.Total)
		fmt.Println(etlJobs.Count)
	}

	// stop the ETL Job
	if err := client.StopETL(projectName, etlJobName); err != nil {
		fmt.Println(err)
	}

	// start the ETL Job
	if err := client.StartETL(projectName, etlJobName); err != nil {
		fmt.Println(err)
	}
}

func getETLJob(etlJobName string, etlScript string) sls.ETL {
	// configuration for ETL output target (sink); you may have one or more sink configurations
	sink := sls.ETLSink{
		Name:            "target0",
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		Endpoint:        endpoint,
		Project:         projectName,
		DataSets: []string{
			"__UNNAMED__",
		},
		Logstore: "target_logstore_name",
	}

	config := sls.ETLConfiguration{
		Version:         2,
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		Logstore:        logStoreName,
		FromTime:        time.Now().Unix(),
		Script:          etlScript,
		Lang:            "SPL",
		Parameters:      map[string]string{},
		ETLSinks:        []sls.ETLSink{sink},
	}

	schedule := sls.ETLSchedule{
		Type: "Resident",
	}

	etljob := sls.ETL{
		Configuration: config,
		DisplayName:   "ETL Job DisplayName",
		Description:   "This ETL job is created by aliyun-log-go-sdk",
		Name:          etlJobName,
		Schedule:      schedule,
		Type:          "ETL",
	}
	return etljob
}
