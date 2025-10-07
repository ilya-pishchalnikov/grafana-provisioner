package grafana

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ApiClient represents Grafana API client
type ApiClient struct {
	URL        string
	Token      string
	HttpClient *http.Client
	Headers    map[string]string
	Retries    int
	RetryDelay time.Duration
	Logger     *slog.Logger
}

// NewClient creates a new Grafana API client
func NewClient(params ClientParams, logger *slog.Logger) *ApiClient {
	// Simple defaults
	if params.Timeout == 0 {
		params.Timeout = 30 * time.Second
	}
	if params.Retries < 0 {
		params.Retries = 3
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 5 * time.Second
	}

	client := &ApiClient{
		URL:   strings.TrimSuffix(params.URL, "/"),
		Token: params.Token,
		HttpClient: &http.Client{
			Timeout: params.Timeout,
		},
		Retries:    params.Retries,
		RetryDelay: params.RetryDelay,
		Logger:     logger,
	}

	client.setDefaultHeaders()
	return client
}

// setDefaultHeaders sets default HTTP headers for API requests
func (apiClient *ApiClient) setDefaultHeaders() {
	if apiClient.Headers == nil {
		apiClient.Headers = make(map[string]string)
	}
	apiClient.Headers["Accept"] = "application/json"
	apiClient.Headers["Content-Type"] = "application/json"
	apiClient.Headers["Authorization"] = "Bearer " + apiClient.Token
}


func (client *ApiClient) GetDataSources(log *slog.Logger) ([]DataSource, error) {
	// Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/datasources", client.URL)

	// Execute the request using retries
	body, err := client.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Десериализация в промежуточную структуру для извлечения database из jsonData
	var rawDataSources []struct {
		ID        int                    `json:"id"`
		UID       string                 `json:"uid"`
		Name      string                 `json:"name"`
		Type      string                 `json:"type"`
		URL       string                 `json:"url"`
		IsDefault bool                   `json:"isDefault"`
		Datebase  string                 `json:"database"`
		JSONData  map[string]interface{} `json:"jsonData"`
	}
	
	if err := json.Unmarshal(body, &rawDataSources); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data sources response: %w", err)
	}

	// Преобразование в конечную структуру DataSource с извлечением database
	dataSources := make([]DataSource, len(rawDataSources))
	for i, rawSource := range rawDataSources {
		dataSources[i] = DataSource{
			ID:        rawSource.ID,
			UID:       rawSource.UID,
			Name:      rawSource.Name,
			Type:      rawSource.Type,
			URL:       rawSource.URL,
			IsDefault: rawSource.IsDefault,
			Database:  rawSource.Datebase,
		}
	}

	log.Info("grafana datasources request successfully parsed")

	// Return the struct
	return dataSources, nil
}

// CreateDataSource sends a POST request to create a new data source.
func (client *ApiClient) CreateDataSource(ds *PostgreSQLDataSourceModel) (*CreateDataSourceResponse, error) {
	client.Logger.Info("Creating new data source", "name", ds.Name)

	// Создаем правильную структуру для Grafana API
	requestData := map[string]interface{}{
		"name":      ds.Name,
		"type":      ds.Type,
		"access":    ds.Access,
		"url":       ds.URL,
		"database":  ds.Database,
		"user":      ds.User,
		"isDefault": ds.IsDefault,
		"jsonData": map[string]interface{}{
			"sslmode":         ds.SSLMode,
			"postgresVersion": 1300, // Укажите версию PostgreSQL
			"timescaledb":     false,
		},
		"secureJsonData": map[string]string{
			"password": ds.Password,
		},
	}

	url := client.URL + "/api/datasources"
	data, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data source model: %w", err)
	}

	respBody, err := client.doRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("data source creation failed: %w", err)
	}

	var response CreateDataSourceResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data source creation response: %w", err)
	}

	client.Logger.Info("Data source successfully created", "name", response.Datasource.Name, "id", response.Datasource.ID)
	return &response, nil
}

// ImportDashboard sends a POST request to import a dashboard.
func (client *ApiClient) ImportDashboard(request *DashboardImportRequest) error {
	client.Logger.Info("Importing dashboard", "overwrite", request.Overwrite)

	url := client.URL + "/api/dashboards/import"
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard import request: %w", err)
	}

	_, err = client.doRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("dashboard import failed: %w", err)
	}

	client.Logger.Info("Dashboard successfully imported")
	return nil
}

// doRequest handles the actual HTTP request with retries
func (client *ApiClient) doRequest(method, url string, body io.Reader) ([]byte, error) {
	var lastErr error
	for i := 0; i < client.Retries; i++ {
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		for key, value := range client.Headers {
			req.Header.Set(key, value)
		}

		resp, err := client.HttpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request failed on attempt %d: %w", i+1, err)
			client.Logger.Warn("Grafana API request failed, retrying...", "error", lastErr.Error(), "attempt", i+1)
			time.Sleep(client.RetryDelay)
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success
			return respBody, nil
		}

		// Handle error response from API
		errorMsg := fmt.Sprintf("Grafana API error (Status %d) on attempt %d: %s", resp.StatusCode, i+1, string(respBody))
		lastErr = errors.New(errorMsg)
		client.Logger.Warn("Grafana API returned error, retrying...", "error", errorMsg, "attempt", i+1)

		// Rewind body if it's a seekable buffer (for retry)
		// if body, ok := body.(*bytes.Buffer); ok {
		// 	body = bytes.NewBuffer(body.Bytes())
		// }
		time.Sleep(client.RetryDelay)
	}

	return nil, fmt.Errorf("failed to execute request after %d attempts: %w", client.Retries, lastErr)
}

// GetFolders fetches the list of all existing dashboard folders
func (client *ApiClient) GetFolders(log *slog.Logger) ([]FolderResponse, error) {
	// Construct the full API URL for folders
	endpoint := fmt.Sprintf("%s/api/folders", client.URL)

	// Execute the request using retries
	body, err := client.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response body into a slice of FolderResponse
	var folders []FolderResponse
	
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal folders response: %w", err)
	}

	log.Info("grafana folders request successfully parsed")

	// Return the struct
	return folders, nil
}

// CreateFolder sends a POST request to create a new folder.
func (client *ApiClient) CreateFolder(title string, log *slog.Logger) (*FolderResponse, error) {
	client.Logger.Info("Creating new folder", "title", title)

	requestData := CreateFolderRequest{
		Title: title,
	}

	url := client.URL + "/api/folders"
	data, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal folder model: %w", err)
	}

	respBody, err := client.doRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		// Grafana API returns 409 if folder with the same name already exists.
		if strings.Contains(err.Error(), "Status 409") {
			client.Logger.Warn("Folder already exists (409 Conflict), this is treated as success for provisioning", "title", title)

			return nil, fmt.Errorf("folder creation failed (409 Conflict): folder with title '%s' already exists", title)
		}
		return nil, fmt.Errorf("folder creation failed: %w", err)
	}

	var response FolderResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal folder creation response: %w", err)
	}

	client.Logger.Info("Folder successfully created", "title", response.Title, "uid", response.UID)
	return &response, nil
}

// SearchDashboards fetches a list of all existing dashboards and folders from the /api/search endpoint.
func (client *ApiClient) SearchDashboards(log *slog.Logger) ([]DashboardSearchResponse, error) {
	// Конструируем полный URL API для поиска дашбордов.
	endpoint := fmt.Sprintf("%s/api/search", client.URL)

	// Выполняем запрос с использованием повторных попыток
	body, err := client.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Десериализуем тело ответа в срез DashboardResponse
	var searchResults []DashboardSearchResponse
	
	if err := json.Unmarshal(body, &searchResults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboard search response: %w", err)
	}

	log.Info("grafana dashboard search request successfully parsed")

	return searchResults, nil
}

// FindFirstDashboardByFolderAndName searches for a dashboard by its title and the title of its containing folder.
func (client *ApiClient) FindFirstDashboardByFolderAndName(name string, folder string, log *slog.Logger) (DashboardSearchResponse, error) {
	log.Info("Searching for dashboard", "name", name, "folder", folder)
	
	searchResults, err := client.SearchDashboards(log)
	if err != nil {
		return DashboardSearchResponse{}, fmt.Errorf("failed to search dashboards: %w", err)
	}

	// Итерируемся по результатам, чтобы найти дашборд, который соответствует обоим критериям
	for _, result := range searchResults {
		// 1. Должен быть дашбордом (type "dash-db")
		if result.Type != "dash-db" {
			continue
		}
		
		// 2. Должен совпадать по имени (Title)
		if result.Title == name {
			// 3. Должен совпадать по имени папки (FolderTitle).
			// Специальная обработка для папки "General" (Общие): Grafana API возвращает FolderTitle="" для дашбордов в "General"
			isGeneralFolder := strings.EqualFold(folder, "General") && (result.FolderTitle == "" || strings.EqualFold(result.FolderTitle, "General"))
			isSpecificFolder := result.FolderTitle == folder

			if isSpecificFolder || isGeneralFolder {
				log.Info("Dashboard found", "name", name, "folder", result.FolderTitle)
				return result, nil
			}
		}
	}

	return DashboardSearchResponse{}, nil
}