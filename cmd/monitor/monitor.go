package core

import (
	"time"

	"push_arch_bin_sync/internal/metrics"
	models "push_arch_bin_sync/internal/models/config"
	"push_arch_bin_sync/internal/models/table"
	binlog_service "push_arch_bin_sync/internal/services/binlog"
	monitor_service "push_arch_bin_sync/internal/services/monitor"
	"push_arch_bin_sync/internal/services/replicator"
	"push_arch_bin_sync/internal/utils"
	"push_arch_bin_sync/pkg/database"

	cc "push_arch_bin_sync/internal/constants"
)

type MonitorServer struct {
	config *models.MonitorConfig
}

func NewMonitorServer(config *models.MonitorConfig) *MonitorServer {
	return &MonitorServer{config: config}
}

func (s *MonitorServer) Run() {
	metrics.Serve(s.config.MetricsAddr)

	utils.CheckServiceUp("Prometheus", s.config.PrometheusHealthURL, 5*time.Second)
	utils.CheckServiceUp("Grafana", s.config.GrafanaHealthURL, 5*time.Second)

	tlsConfig, tlsErr := utils.SetupTLS(s.config.CertPath)
	if tlsErr != nil {
		utils.LogError("TLS setup failed: %v", tlsErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}
	if s.config.DestTLSServerName != "" {
		tlsConfig.ServerName = s.config.DestTLSServerName
	}

	// binlog: reads row-level changes for our 15 monitored tables
	binlogMonitor, blErr := binlog_service.NewBinaryLogMonitor(
		s.config.DBHost,
		s.config.DBPort,
		s.config.BinlogUser,
		s.config.BinlogPass,
		table.GetTableNames(),
	)

	if blErr != nil {
		utils.LogError("Binary log monitor creation failed: %v", blErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	// destination: DigitalOcean Managed MySQL that mirrors the source tables
	destDB, dbErr := database.NewDestinationConnector(database.DestinationDBConfig{
		Host:     s.config.DestDBHost,
		Port:     s.config.DestDBPort,
		User:     s.config.DestDBUser,
		Password: s.config.DestDBPassword,
		DBName:   s.config.DestDBName,
	}, tlsConfig)

	if dbErr != nil {
		utils.LogError("Destination database connection failed: %v", dbErr)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	mysqlReplicator := replicator.NewMySQLReplicator(destDB)

	mon := monitor_service.NewMonitor(s.config, binlogMonitor, mysqlReplicator)
	defer mon.Close()

	mon.Run()
}
