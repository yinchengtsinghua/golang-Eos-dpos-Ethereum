
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package tracing

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	jaeger "github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	cli "gopkg.in/urfave/cli.v1"
)

var Enabled bool = false

//
const TracingEnabledFlag = "tracing"

var (
	Closer io.Closer
)

var (
	TracingFlag = cli.BoolFlag{
		Name:  TracingEnabledFlag,
		Usage: "Enable tracing",
	}
	TracingEndpointFlag = cli.StringFlag{
		Name:  "tracing.endpoint",
		Usage: "Tracing endpoint",
		Value: "0.0.0.0:6831",
	}
	TracingSvcFlag = cli.StringFlag{
		Name:  "tracing.svc",
		Usage: "Tracing service name",
		Value: "swarm",
	}
)

//
var Flags = []cli.Flag{
	TracingFlag,
	TracingEndpointFlag,
	TracingSvcFlag,
}

//
func init() {
	for _, arg := range os.Args {
		if flag := strings.TrimLeft(arg, "-"); flag == TracingEnabledFlag {
			Enabled = true
		}
	}
}

func Setup(ctx *cli.Context) {
	if Enabled {
		log.Info("Enabling opentracing")
		var (
			endpoint = ctx.GlobalString(TracingEndpointFlag.Name)
			svc      = ctx.GlobalString(TracingSvcFlag.Name)
		)

		Closer = initTracer(endpoint, svc)
	}
}

func initTracer(endpoint, svc string) (closer io.Closer) {
//
//
	cfg := jaegercfg.Configuration{
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  endpoint,
		},
	}

//
//
//
	jLogger := jaegerlog.StdLogger
//

//
	closer, err := cfg.InitGlobalTracer(
		svc,
		jaegercfg.Logger(jLogger),
//
//
	)
	if err != nil {
		log.Error("Could not initialize Jaeger tracer", "err", err)
	}

	return closer
}
