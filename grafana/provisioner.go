package grafana

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// RunProvisioning executes the full provisioning workflow
func RunProvisioning(cfg Config, log *slog.Logger) error {
	log.Info("Starting Grafana provisioning process")
	client := NewClient(cfg.Grafana, log)

	// 1. Wait for Grafana API availability
	if err := waitForGrafanaAPI(client); err != nil {
		return fmt.Errorf("grafana API did not become available: %w", err)
	}

	// 2. Provision Data Source
	dsResponse, err := provisionDataSource(client, cfg, log)
	if err != nil {
		return fmt.Errorf("data source provisioning failed: %w", err)
	}

	// 3. Provision Folder
	folderResponse, err := provisionFolder(client, cfg, log)
	if err != nil {
		return fmt.Errorf("folder provisioning failed: %w", err)
	}

	// 4. Provision Dashboard
	if err := provisionDashboard(client, cfg, dsResponse.Datasource.UID, folderResponse.UID, log); err != nil {
		return fmt.Errorf("dashboard provisioning failed: %w", err)
	}

	log.Info("Grafana provisioning completed successfully")
	return nil
}

// Helper to wait for Grafana API to be ready
func waitForGrafanaAPI(client *ApiClient) error {
	client.Logger.Info("Waiting for Grafana API to become ready...")

	url := client.URL + "/api/health"
	for i := 0; i < client.Retries; i++ {
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := client.HttpClient.Do(req)

		if err == nil && resp.StatusCode == 200 {
			client.Logger.Info("Grafana API is ready")
			resp.Body.Close()
			return nil
		}

		if resp==nil {			
			client.Logger.Warn("Grafana API not ready, retrying...", "error", err, "attempt", i+1)
		} else {			
			client.Logger.Warn("Grafana API not ready, retrying...", "error", err, "status", resp.StatusCode, "attempt", i+1)
		}
		
		if resp == nil {
			client.Logger.Warn("Grafana API not ready, retrying...", "error", err, "attempt", i+1)
		} else {
			client.Logger.Warn("Grafana API not ready, retrying...", "error", err, "status", resp.StatusCode, "attempt", i+1)
		}
		
		time.Sleep(client.RetryDelay)
	}

	return fmt.Errorf("failed to reach Grafana API after %d attempts", client.Retries)
}

// Helper to create the data source
func provisionDataSource(client *ApiClient, cfg Config, log *slog.Logger) (*CreateDataSourceResponse, error) {
	// Получение списка всех существующих источников данных
	existingSources, err := client.GetDataSources(log)
    if err != nil {
        return nil, fmt.Errorf("failed to list existing data sources: %w", err)
    }

    // Проверка, существует ли источник данных с тем же типом, URL и базой данных
    for _, source := range existingSources {
        if source.Type == cfg.DataSource.Type && source.URL == cfg.DataSource.URL && source.Database == cfg.DataSource.Database {
            log.Info(fmt.Sprintf("data source of type '%s' with URL '%s' and database '%s' already exists (ID: %d). Skipping creation.", 
                source.Type, source.URL, source.Database, source.ID))

			return &CreateDataSourceResponse{
				CreateDataSourceResponseDatasource {
					ID: source.ID,
					UID: source.UID,
					Name: source.Name,
					Message: "Already exists",
				},
			}, nil
        }
    }
	
	// Проверка на дублирование имени и инкремент
    sourceToCreate := cfg.DataSource
    baseName := cfg.DataSource.Name

	for i := 0; ; i++ {
        currentName := baseName
        if i > 0 {
            currentName = fmt.Sprintf("%s_%d", baseName, i)
        }
        
        // Проверяем, существует ли источник с текущим именем
        nameConflict := false
        for _, source := range existingSources {
            if source.Name == currentName {
                nameConflict = true
                break
            }
        }

        if !nameConflict {
            sourceToCreate.Name = currentName
            break
        }
    }
	
	dsModel := &PostgreSQLDataSourceModel{
		Name:      sourceToCreate.Name,
		Type:      "grafana-postgresql-datasource",
		Access:    "direct",
		URL:       sourceToCreate.URL,
		Database:  sourceToCreate.Database,
		User:      sourceToCreate.User,
		Password:  sourceToCreate.Password,
		SSLMode:   sourceToCreate.SSLMode,
		IsDefault: false,
	}

	// Attempt to create the data source
	resp, err := client.CreateDataSource(dsModel)

	// Grafana API returns 409 if data source with the same name already exists.
	// We treat this as success because the goal (existence) is met.
	if err != nil && strings.Contains(err.Error(), "Status 409") {
		log.Warn("Data source already exists (409 Conflict), continuing...", "name", dsModel.Name)

		return &CreateDataSourceResponse{
			CreateDataSourceResponseDatasource {
				Name: dsModel.Name,
				Message: "Data source already exists (409 Conflict)",
			},
		}, nil
	}

	return resp, err
}


// Helper to create the folder
func provisionFolder(client *ApiClient, cfg Config, log *slog.Logger) (*FolderResponse, error) {
	// Получение списка всех существующих источников данных
	existingFolders, err := client.GetFolders(log)
    if err != nil {
        return nil, fmt.Errorf("failed to list existing data sources: %w", err)
    }

    // Проверка, существует ли источник данных с тем же именем
    for _, folder := range existingFolders {
        if folder.Title == cfg.Dashboard.Folder {
            log.Info(fmt.Sprintf("folder '%s' already exists (ID: %d). Skipping creation.", folder.Title, folder.ID))

			return &FolderResponse{
				ID:    folder.ID,
				UID:   folder.UID,
				Title: folder.Title,
			}, nil
        }
    }

	// Attempt to create the folder
	return client.CreateFolder(cfg.Dashboard.Folder, log)
}

// Helper to import the dashboard
func provisionDashboard(client *ApiClient, cfg Config, dsUID string, folderUID string, log *slog.Logger) error {
	log.Info("Reading dashboard file", "file", cfg.Dashboard.File)
	data, err := os.ReadFile(cfg.Dashboard.File)
	if err != nil {
		return fmt.Errorf("failed to read dashboard file %s: %w", cfg.Dashboard.File, err)
	}

	var rawDashboard DashboardJSON
	if err := json.Unmarshal(data, &rawDashboard); err != nil {
		return fmt.Errorf("failed to parse dashboard JSON: %w", err)
	}

	existingDashboard, err := client.FindFirstDashboardByFolderAndName(cfg.Dashboard.Name, cfg.Dashboard.Folder, log)
	if err != nil {
		return fmt.Errorf("failed to find existsting dashboard: %w", err)
	}

	rawDashboard["title"] = cfg.Dashboard.Name
	rawDashboard["id"] = existingDashboard.ID
	rawDashboard["uid"] = existingDashboard.UID


	inputValues := map[string]string{
        cfg.Dashboard.ImportVar: dsUID,
    }

	// 2. Prepare inputs from the exported data or provided values
	var inputs []interface{}
	if exportedInputs, exists := rawDashboard["__inputs"]; exists {
		inputsSlice, ok := exportedInputs.([]interface{})
		if ok {
			inputs = processInputs(inputsSlice, inputValues)
		}
	}

	importRequest := &DashboardImportRequest{
		Dashboard: rawDashboard,
		Inputs: inputs,
		FolderUID: folderUID,
		Overwrite: true, // Always overwrite to apply latest changes
		Message:   "Automated provisioning by grafana-provisioner",
	}

	return client.ImportDashboard(importRequest)
}

// processInputs processes input variables and sets their values
func processInputs(inputs []interface{}, inputValues map[string]string) []interface{} {
    var processedInputs []interface{}

    for _, input := range inputs {
        inputMap, ok := input.(map[string]interface{})
        if !ok {
            continue
        }

        // Get input name
        name, exists := inputMap["name"].(string)
        if !exists {
            continue
        }

        // Set value from provided values or use default
        if value, exists := inputValues[name]; exists {
            inputMap["value"] = value
        } else {
            // Try to use the default value from the input
            if currentValue, exists := inputMap["value"]; exists {
                inputMap["value"] = currentValue
            }
        }

        processedInputs = append(processedInputs, inputMap)
    }

    return processedInputs
}