
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

package metrics

import (
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/influxdb"
	"github.com/ethereum/go-ethereum/swarm/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	metricsEnableInfluxDBExportFlag = cli.BoolFlag{
		Name:  "metrics.influxdb.export",
		Usage: "Enable metrics export/push to an external InfluxDB database",
	}
	metricsInfluxDBEndpointFlag = cli.StringFlag{
		Name:  "metrics.influxdb.endpoint",
		Usage: "Metrics InfluxDB endpoint",
Value: "http://
	}
	metricsInfluxDBDatabaseFlag = cli.StringFlag{
		Name:  "metrics.influxdb.database",
		Usage: "Metrics InfluxDB database",
		Value: "metrics",
	}
	metricsInfluxDBUsernameFlag = cli.StringFlag{
		Name:  "metrics.influxdb.username",
		Usage: "Metrics InfluxDB username",
		Value: "",
	}
	metricsInfluxDBPasswordFlag = cli.StringFlag{
		Name:  "metrics.influxdb.password",
		Usage: "Metrics InfluxDB password",
		Value: "",
	}
//
//
//
//
	metricsInfluxDBHostTagFlag = cli.StringFlag{
		Name:  "metrics.influxdb.host.tag",
		Usage: "Metrics InfluxDB `host` tag attached to all measurements",
		Value: "localhost",
	}
)

//
var Flags = []cli.Flag{
	utils.MetricsEnabledFlag,
	metricsEnableInfluxDBExportFlag,
	metricsInfluxDBEndpointFlag, metricsInfluxDBDatabaseFlag, metricsInfluxDBUsernameFlag, metricsInfluxDBPasswordFlag, metricsInfluxDBHostTagFlag,
}

func Setup(ctx *cli.Context) {
	if gethmetrics.Enabled {
		log.Info("Enabling swarm metrics collection")
		var (
			enableExport = ctx.GlobalBool(metricsEnableInfluxDBExportFlag.Name)
			endpoint     = ctx.GlobalString(metricsInfluxDBEndpointFlag.Name)
			database     = ctx.GlobalString(metricsInfluxDBDatabaseFlag.Name)
			username     = ctx.GlobalString(metricsInfluxDBUsernameFlag.Name)
			password     = ctx.GlobalString(metricsInfluxDBPasswordFlag.Name)
			hosttag      = ctx.GlobalString(metricsInfluxDBHostTagFlag.Name)
		)

//
		go gethmetrics.CollectProcessMetrics(2 * time.Second)

		if enableExport {
			log.Info("Enabling swarm metrics export to InfluxDB")
			go influxdb.InfluxDBWithTags(gethmetrics.DefaultRegistry, 10*time.Second, endpoint, database, username, password, "swarm.", map[string]string{
				"host": hosttag,
			})
		}
	}
}
