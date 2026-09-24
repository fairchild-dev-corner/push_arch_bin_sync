package config

import (
	"errors"
	"strconv"

	models "push_arch_bin_sync/internal/models/config"
)

/**
* Binary Log Monitor Configuration & Enviroment Setup
* @Description: Define Setup Configuration for the source (binlog) connection
* and the destination (mirror) database connection.
**/
func GetMonitorConfig() (*models.MonitorConfig, error) {
	if Envs.DBHost == "" || Envs.BinlogUser == "" {
		return nil, errors.New("missing required source config variables")
	}
	if Envs.DestDBHost == "" || Envs.DestDBUser == "" || Envs.DestDBName == "" {
		return nil, errors.New("missing required destination config variables")
	}
	if Envs.CertPath == "" {
		return nil, errors.New("missing required CERT_PATH for destination TLS")
	}

	port, err := strconv.Atoi(Envs.DBPort)
	if err != nil || port <= 0 {
		port = 3306
	}

	destPort, err := strconv.Atoi(Envs.DestDBPort)
	if err != nil || destPort <= 0 {
		destPort = 3306
	}

	metricsPort := Envs.MetricsPort
	if metricsPort == "" {
		metricsPort = "9308"
	}

	prometheusHealthURL := Envs.PrometheusHealthURL
	if prometheusHealthURL == "" {
		prometheusHealthURL = "http://localhost:9090/-/ready"
	}

	grafanaHealthURL := Envs.GrafanaHealthURL
	if grafanaHealthURL == "" {
		grafanaHealthURL = "http://localhost:3000/api/health"
	}

	lokiURL := Envs.LokiURL
	if lokiURL == "" {
		lokiURL = "http://localhost:3100"
	}

	return models.NewMonitorConfig(
		Envs.DBHost,
		uint16(port),
		Envs.BinlogUser,
		Envs.BinlogPassword,
		Envs.DBName,
		Envs.DestDBHost,
		uint16(destPort),
		Envs.DestDBUser,
		Envs.DestDBPassword,
		Envs.DestDBName,
		Envs.CertPath,
		":"+metricsPort,
		prometheusHealthURL,
		grafanaHealthURL,
		lokiURL,
		Envs.DestTLSServerName,
	), nil
}
