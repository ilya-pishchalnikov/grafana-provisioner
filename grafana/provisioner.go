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
	_, err := provisionDataSources(client, cfg, log)
	if err != nil {
		return fmt.Errorf("data source provisioning failed: %w", err)
	}

	// 3. Provision Folders from config and create mapping
	if err := provisionFolders(client, &cfg, log); err != nil {
		return fmt.Errorf("folder provisioning failed: %w", err)
	}

	// 4. Provision Dashboards (handle multiple dashboards from config)
	if err := provisionDashboards(client, cfg, log); err != nil {
		return fmt.Errorf("dashboard provisioning failed: %w", err)
	}

	log.Info("Grafana provisioning completed successfully")
	return nil
}

// provisionDashboards iterates over the configured dashboards and provisions each one.
func provisionDashboards(client *ApiClient, cfg Config, log *slog.Logger) error {
	if len(cfg.Dashboards) == 0 {
		log.Info("No dashboards configured for provisioning, skipping dashboard creation.")
		return nil
	}

	log.Info("Provisioning Grafana dashboards")
	for _, dashboardConfig := range cfg.Dashboards {
		// 1. Validate and get folder UID for the dashboard
		dashboardFolderUID, err := getDashboardFolderUID(cfg, dashboardConfig, log)
		if err != nil {
			return fmt.Errorf("dashboard folder validation failed for dashboard '%s': %w", dashboardConfig.Name, err)
		}

		// 2. Get dashboard data source
		dashboardDataSource, err := client.GetDataSource(dashboardConfig.DataSource)
		if err != nil {
			return fmt.Errorf("dashboard dataSource '%s' not found for dashboard '%s': %w", dashboardConfig.DataSource, dashboardConfig.Name, err)
		}

		// 3. Provision the specific dashboard
		if err := provisionDashboard(client, dashboardConfig, dashboardDataSource.UID, dashboardFolderUID, log); err != nil {
			return fmt.Errorf("dashboard provisioning failed for dashboard '%s': %w", dashboardConfig.Name, err)
		}
	}
	log.Info("All configured dashboards provisioned.")
	return nil
}


// provisionFolders creates all folders defined in the config and stores their IDs/UIDs in Config.FoldersMapping.
func provisionFolders(client *ApiClient, cfg *Config, log *slog.Logger) error {
	cfg.FoldersMapping = make(map[string]FolderMapping)
	
	// Create a map of folders from the main config (which contains the names)
	folderConfigs := cfg.Folders

	// Ensure the list is not empty
	if len(folderConfigs) == 0 {
		log.Info("No folders configured for provisioning, skipping folder creation.")
		return nil
	}

	log.Info("Provisioning Grafana folders")
	for _, folderConfig := range folderConfigs {
		resp, err := client.CreateFolderIfNotExists(folderConfig.Name, log)
		if err != nil {
			return fmt.Errorf("failed to provision folder '%s': %w", folderConfig.Name, err)
		}
		
		// Store the mapping for later use (e.g., dashboard creation)
		cfg.FoldersMapping[resp.Title] = FolderMapping{
			ID:    resp.ID,
			UID:   resp.UID,
			Title: resp.Title,
		}
	}

	log.Info("All configured folders provisioned and mapped.")
	return nil
}

// getDashboardFolderUID validates that the folder required for the dashboard exists in the mapping
// and returns its UID. It is modified to accept a single Dashboard config.
func getDashboardFolderUID(cfg Config, dashboardConfig Dashboard, log *slog.Logger) (string, error) {
	requiredFolder := dashboardConfig.Folder

	// Handle case: Dashboard goes into the 'General' folder (Grafana default)
	if strings.EqualFold(requiredFolder, "General") {
		// Check if General was mapped (it might have been mapped in provisionFolders)
		if mapping, ok := cfg.FoldersMapping["General"]; ok {
			log.Info("Using 'General' folder UID from mapping", "uid", mapping.UID)
			return mapping.UID, nil
		}
		
		// If still not mapped and retrieval failed, rely on API default behavior.
		log.Info("Using default folder UID for 'General' folder (empty string or General)")
		return "", nil 
	}
	
	// Check if the required folder is in our mapping
	mapping, ok := cfg.FoldersMapping[requiredFolder]
	if !ok {
		// This is the required exception/error case: folder is not in the config list
		return "", fmt.Errorf("dashboard folder '%s' is not defined in the 'folders' configuration list. Please add it to provision it", requiredFolder)
	}

	return mapping.UID, nil
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

func provisionDataSources(client *ApiClient, cfg Config, log *slog.Logger) (*[]CreateDataSourceResponse, error) {
	existingSources, err := client.GetDataSources(log)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing data sources: %w", err)
	}

	sourceResponses := []CreateDataSourceResponse{}

	for _, dataSource := range cfg.DataSources {
		sourceResponce, err := provisionDataSource(client, dataSource, existingSources, log)
		if err != nil {
			return nil, fmt.Errorf("failed to provision datasource '%s': %w", dataSource.Name, err)
		}

		sourceResponses = append(sourceResponses, *sourceResponce);
	}

	return &sourceResponses, nil
}

// Helper to create the data source
func provisionDataSource(client *ApiClient, dataSource DataSource, existingSources []DataSource, log *slog.Logger) (*CreateDataSourceResponse, error) {
    // Check if a data source with the same type, URL and database already exists
    for _, source := range existingSources {
        if source.Type == dataSource.Type && source.URL == dataSource.URL && source.Database == dataSource.Database {
            log.Info(fmt.Sprintf("data source of type '%s' with URL '%s' and database '%s' already exists (ID: %d). Skipping creation.", 
                source.Type, source.URL, source.Database, source.ID))

			return &CreateDataSourceResponse{
				Datasource: CreateDataSourceResponseDatasource {
					ID: source.ID,
					UID: source.UID,
					Name: source.Name,
					Message: "Already exists",
				},
			}, nil
        }
    }
	
	// Check for name duplication and increment
    sourceToCreate := dataSource
    baseName := dataSource.Name

	for i := 0; ; i++ {
        currentName := baseName
        if i > 0 {
            currentName = fmt.Sprintf("%s_%d", baseName, i)
        }
        
        // Check if a source with the current name exists
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
			Datasource: CreateDataSourceResponseDatasource {
				Name: dsModel.Name,
				Message: "Data source already exists (409 Conflict)",
			},
		}, nil
	}

	return resp, err
}

// Helper to import the dashboard
func provisionDashboard(client *ApiClient, cfg Dashboard, dsUID string, folderUID string, log *slog.Logger) error {
	log.Info("Reading dashboard file", "file", cfg.File)
	data, err := os.ReadFile(cfg.File)
	if err != nil {
		return fmt.Errorf("failed to read dashboard file %s: %w", cfg.File, err)
	}

	var rawDashboard DashboardJSON
	if err := json.Unmarshal(data, &rawDashboard); err != nil {
		return fmt.Errorf("failed to parse dashboard JSON: %w", err)
	}

	existingDashboard, err := client.FindFirstDashboardByFolderAndName(cfg.Name, cfg.Folder, log)
	if err != nil {
		return fmt.Errorf("failed to find existsting dashboard: %w", err)
	}

	rawDashboard["title"] = cfg.Name
	rawDashboard["id"] = existingDashboard.ID
	rawDashboard["uid"] = existingDashboard.UID


	// Get the target folder UID. If 'folderUID' is empty (for 'General' folder), the API handles it.
	// If the dashboard folder is 'General', we pass an empty folderUID to the import API call.
	if strings.EqualFold(cfg.Folder, "General") {
		folderUID = "" // Grafana API uses empty/nil folder UID for the 'General' folder
	}
	
	inputValues := map[string]string{
        cfg.ImportVar: dsUID,
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