package main

import (
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/example/util"
)

func main() {
	fmt.Println("Tag Resource")
	resouceId := sls.GenResourceId(util.ProjectName, util.LogStoreName)
	resourceTags := sls.NewResourceTags("logstore", resouceId, []sls.ResourceTag{
		{
			Key:   "the-tag",
			Value: "aliyun-log-go-sdk",
		},
		{
			Key:   "the-tag-2",
			Value: "aliyun log go sdk",
		},
	})
	err := util.Client.TagResources(util.ProjectName, resourceTags)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Tag Resource success")
	}
	listTags("logstore")

	fmt.Println("UnTag Resource")
	resouceUnTags := sls.NewResourceUnTags("logstore", resouceId, []string{"the-tag"})

	err = util.Client.UnTagResources(util.ProjectName, resouceUnTags)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("UnTag Resource success")
	}
	listTags("logstore")
}

func listTags(resourceType string) {
	fmt.Println("tag list: ")
	respTags, _, err := util.Client.ListTagResources(
		util.ProjectName,
		resourceType,
		[]string{sls.GenResourceId(util.ProjectName, util.LogStoreName)},
		[]sls.ResourceFilterTag{},
		"")
	if err != nil {
		panic(err)
	}
	for _, tag := range respTags {
		fmt.Printf(" resourceType : %s, resourceID : %s, tagKey : %s, tagValue : %s\n",
			tag.ResourceType,
			tag.ResourceID,
			tag.TagKey,
			tag.TagValue)
	}
}
