package collector

import (
	"context"
	"net"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const ConnectionInfoName = "connection_info"

var rdsRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)

type ConnectionInfoArguments struct {
	DSN      string
	Registry *prometheus.Registry
}

type ConnectionInfo struct {
	DSN        string
	Registry   *prometheus.Registry
	InfoMetric *prometheus.GaugeVec

	running *atomic.Bool
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "db_instance_identifier"})

	args.Registry.MustRegister(infoMetric)

	return &ConnectionInfo{
		DSN:        args.DSN,
		Registry:   args.Registry,
		InfoMetric: infoMetric,
		running:    &atomic.Bool{},
	}, nil
}

func (c *ConnectionInfo) Name() string {
	return ConnectionInfoName
}

func (c *ConnectionInfo) Start(ctx context.Context) error {
	cfg, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		return err
	}

	c.running.Store(true)

	var (
		providerName         = "unknown"
		providerRegion       = "unknown"
		dbInstanceIdentifier = "unknown"
	)

	host, _, err := net.SplitHostPort(cfg.Addr)
	if err == nil && host != "" {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			providerName = "aws"
			matches := rdsRegex.FindStringSubmatch(host)
			if len(matches) > 3 {
				dbInstanceIdentifier = matches[1]
				providerRegion = matches[3]
			}
		}
	}

	c.InfoMetric.WithLabelValues(providerName, providerRegion, dbInstanceIdentifier).Set(1)
	return nil
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	c.Registry.Unregister(c.InfoMetric)
	c.running.Store(false)
}
