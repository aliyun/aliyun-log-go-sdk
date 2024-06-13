## 介绍
cgo-zstd 是基于 cgo 实现的 zstd 压缩库，比纯 go 实现的 zstd 在性能上具有一定优势。  
下面介绍如何开启此扩展来提升 zstd 压缩的性能。  

## 依赖
使用此扩展，需要设置 go env 中的 CGO_ENABLED=1，且环境中已安装了 gcc。  
可以通过以下命令查看当前 CGO_ENABLED 是否打开。

```bash
go env | grep CGO_ENABLED
```

### 全局永久开启
```bash 
go env -w CGO_ENABLED=1
```

### 临时开启
```
CGO_ENABLED=1 go build
```

## 使用方法
开启 cgo-zstd 扩展

```golang
import (
    cgozstd "github.com/aliyun/aliyun-log-go-sdk/extensions/cgo-zstd"
    sls "github.com/aliyun/aliyun-log-go-sdk"
)

compressLevel := 1
sls.DefaultZstdCompressor = cgozstd.NewZstdCompressor(compressLevel)
```


使用 zstd 压缩写入日志的示例
```golang
import (
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/golang/protobuf/proto"
)

func main() {

	client := sls.CreateNormalInterface("endpoint",
		"accessKeyId", "accessKeySecret", "")
	lg := &sls.LogGroup{
		Logs: []*sls.Log{
			{
				Time: proto.Uint32(uint32(time.Now().Unix())),
				Contents: []*sls.LogContent{
					{
						Key:   proto.String("HELLO"),
						Value: proto.String("world"),
					},
				},
			},
		},
	}
	err := client.PostLogStoreLogsV2(
		"your-project",
		"your-logstore",
		&sls.PostLogStoreLogsRequest{
			LogGroup:     lg,
			CompressType: sls.Compress_ZSTD, // 指定压缩方式为 ZSTD
		},
	)
	if err != nil {
		panic(err)
	}

}
```