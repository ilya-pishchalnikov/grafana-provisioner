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


	// 3. Convert config types to grafana provisioner types
	
	dataSources := []grafana.DataSource{}

	for _, dataSourceConfig := range appConfig.DataSources {
		// PostgreSQL is hardcoded for now, type is always grafana-postgresql-datasource
		dataSource := grafana.DataSource {
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

	dashboards := []grafana.Dashboard{}

	for _, dashboardConfig := range appConfig.Dashboards {
		// Convert imports
		dashboardImports := []grafana.DashboardImport{}
		for _, importConfig := range dashboardConfig.Imports {
			dashboardImports = append(dashboardImports, grafana.DashboardImport{
				Name:       importConfig.Name,
				DataSource: importConfig.DataSource,
			})
		}
		
		dashboard := grafana.Dashboard {
			Name:       dashboardConfig.Name,
			Folder:     dashboardConfig.Folder,
			File:       dashboardConfig.File,
			Imports:    dashboardImports,
		}

		dashboards = append(dashboards, dashboard)
	}
	
	folders := []grafana.Folder{}

	for _, folderConfig := range appConfig.Folders {
		folder := grafana.Folder {
			Name: folderConfig.Name,
		}
		folders = append(folders, folder)
	}


	provisionerConfig := grafana.Config{
		Grafana: grafana.ClientParams{
			URL:        appConfig.Grafana.URL,
			Token:      appConfig.Grafana.Token,
			Timeout:    appConfig.Grafana.Timeout.Duration,
			Retries:    appConfig.Grafana.Retries,
			RetryDelay: appConfig.Grafana.RetryDelay.Duration,
		},
		Dashboards: dashboards,
		DataSources: dataSources,
		Folders: folders, // Use the converted slice
		FoldersMapping: nil, // Will be populated in grafana.RunProvisioning
	}

	// 4. Run Provisioning
	if err := grafana.RunProvisioning(provisionerConfig, log); err != nil {
		log.Error("FATAL: Grafana provisioning failed", "error", err)
		os.Exit(1)
	}

	log.Info("Application finished successfully.")
}