package main

import (
	"grafana-provisioner/config"
	"grafana-provisioner/grafana"
	"log/slog"
	"os"
	"strconv"
)

func main() {
	// 1. Load configuration
	appConfig, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("FATAL: Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize logger (using slog)
	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(appConfig.Log.Level)); err != nil {
		slog.Error("FATAL: Invalid log level in config", "level", appConfig.Log.Level)
		os.Exit(1)
	}

	var log *slog.Logger

	if appConfig.Log.Format == "text" {
		logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		})
		log = slog.New(logHandler)
	}
	if appConfig.Log.Format == "json" {
		logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		})
		log = slog.New(logHandler)
	}
	
	slog.SetDefault(log)
	log.Info("Provisioner logger started")


	// 3. Prepare Grafana Provisioning Configuration

	dataSources := []grafana.DataSource{}

	for _, dataSourceConfig := range appConfig.DataSources {
		dataSource := grafana.DataSource{
			Name:      dataSourceConfig.Name,
			Type:      "grafana-postgresql-datasource",
			URL:       dataSourceConfig.Host + ":" + strconv.Itoa(dataSourceConfig.Port),
			Database:  dataSourceConfig.DbName,
			User:      dataSourceConfig.User,
			Password:  dataSourceConfig.Password,
			SSLMode:   dataSourceConfig.SslMode,
			IsDefault: false,
		}

		dataSources = append(dataSources, dataSource)
	}


	provisionerConfig := grafana.Config{
		Grafana: grafana.ClientParams{
			URL:        appConfig.Grafana.URL,
			Token:      appConfig.Grafana.Token,
			Timeout:    appConfig.Grafana.Timeout.Duration,
			Retries:    appConfig.Grafana.Retries,
			RetryDelay: appConfig.Grafana.RetryDelay.Duration,
		},
		Dashboard: grafana.Dashboard{
			Name:       appConfig.Grafana.Dashboard.Name,
			Folder:     appConfig.Grafana.Dashboard.Folder,
			File:       appConfig.Grafana.Dashboard.File,
			DataSource: appConfig.Grafana.Dashboard.DataSource,
			ImportVar:  appConfig.Grafana.Dashboard.ImportVar,
		},
		DataSources: dataSources,
		FoldersMapping: nil, // Will be populated in grafana.RunProvisioning
	}

	// Set the Folders field in the Dashboard struct to the list of folders from appConfig
	// This is a small hack, but it simplifies parameter passing for the new provisionFolders function.
	provisionerConfig.Folders = appConfig.Folders

	// 4. Run Provisioning
	if err := grafana.RunProvisioning(provisionerConfig, log); err != nil {
		log.Error("FATAL: Grafana provisioning failed", "error", err)
		os.Exit(1)
	}

	log.Info("Application finished successfully.")
}