package config

// MonitorConfig holds the settings needed to connect to the source MySQL
// binary log and to write mirrored changes into the destination database.
type MonitorConfig struct {
	DBHost     string
	DBPort     uint16
	BinlogUser string
	BinlogPass string
	DBName     string

	DestDBHost     string
	DestDBPort     uint16
	DestDBUser     string
	DestDBPassword string
	DestDBName     string

	// CertPath is the PEM file path for the destination's pinned CA cert.
	CertPath string

	// MetricsAddr is the address (e.g. ":9308") the Prometheus /metrics
	// endpoint listens on.
	MetricsAddr string

	// PrometheusHealthURL / GrafanaHealthURL are checked at startup to log
	// whether the monitoring stack is up. Purely informational - neither
	// being unreachable stops the daemon from running.
	PrometheusHealthURL string
	GrafanaHealthURL    string

	// LokiURL is the base URL LogError/LogSync push log lines to, for the
	// live "Recent Sync Activity" Grafana panel. Empty disables Loki push -
	// same "observability, not a dependency" treatment as Prometheus/Grafana.
	LokiURL string

	// DestTLSServerName overrides the hostname the destination's TLS cert is
	// verified against. Needed when DestDBHost is a TCP proxy (e.g. NGINX
	// stream) rather than the real DigitalOcean hostname. Empty means verify
	// against DestDBHost.
	DestTLSServerName string
}

func NewMonitorConfig(
	dbHost string,
	dbPort uint16,
	binlogUser string,
	binlogPass string,
	dbName string,
	destDBHost string,
	destDBPort uint16,
	destDBUser string,
	destDBPassword string,
	destDBName string,
	certPath string,
	metricsAddr string,
	prometheusHealthURL string,
	grafanaHealthURL string,
	lokiURL string,
	destTLSServerName string,
) *MonitorConfig {
	return &MonitorConfig{
		DBHost:              dbHost,
		DBPort:              dbPort,
		BinlogUser:          binlogUser,
		BinlogPass:          binlogPass,
		DBName:              dbName,
		DestDBHost:          destDBHost,
		DestDBPort:          destDBPort,
		DestDBUser:          destDBUser,
		DestDBPassword:      destDBPassword,
		DestDBName:          destDBName,
		CertPath:            certPath,
		MetricsAddr:         metricsAddr,
		PrometheusHealthURL: prometheusHealthURL,
		GrafanaHealthURL:    grafanaHealthURL,
		LokiURL:             lokiURL,
		DestTLSServerName:   destTLSServerName,
	}
}
