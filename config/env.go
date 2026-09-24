package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost         string
	DBPort         string
	BinlogUser     string
	BinlogPassword string
	DBName         string

	DestDBHost     string
	DestDBPort     string
	DestDBUser     string
	DestDBPassword string
	DestDBName     string

	CertPath          string
	DestTLSServerName string

	MetricsPort string

	PrometheusHealthURL string
	GrafanaHealthURL    string
	LokiURL             string
}

/*Create Singleton*/
var Envs = initConfig()

func initConfig() Config {

	//reload enviroment var
	godotenv.Load()

	return Config{
		DBHost:         getEnv("DB_HOST", os.Getenv("DB_HOST")),
		DBPort:         getEnv("DB_PORT", os.Getenv("DB_PORT")),
		BinlogUser:     getEnv("BINLOG_USER", os.Getenv("BINLOG_USER")),
		BinlogPassword: getEnv("BINLOG_PASSWORD", os.Getenv("BINLOG_PASSWORD")),
		DBName:         getEnv("DB_NAME", os.Getenv("DB_NAME")),

		DestDBHost:     getEnv("DEST_DB_HOST", os.Getenv("DEST_DB_HOST")),
		DestDBPort:     getEnv("DEST_DB_PORT", os.Getenv("DEST_DB_PORT")),
		DestDBUser:     getEnv("DEST_DB_USER", os.Getenv("DEST_DB_USER")),
		DestDBPassword: getEnv("DEST_DB_PASSWORD", os.Getenv("DEST_DB_PASSWORD")),
		DestDBName:     getEnv("DEST_DB_NAME", os.Getenv("DEST_DB_NAME")),

		CertPath:          getEnv("CERT_PATH", os.Getenv("CERT_PATH")),
		DestTLSServerName: getEnv("DEST_DB_TLS_SERVER_NAME", os.Getenv("DEST_DB_TLS_SERVER_NAME")),

		MetricsPort: getEnv("METRICS_PORT", os.Getenv("METRICS_PORT")),

		PrometheusHealthURL: getEnv("PROMETHEUS_HEALTH_URL", os.Getenv("PROMETHEUS_HEALTH_URL")),
		GrafanaHealthURL:    getEnv("GRAFANA_HEALTH_URL", os.Getenv("GRAFANA_HEALTH_URL")),
		LokiURL:             getEnv("LOKI_URL", os.Getenv("LOKI_URL")),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
