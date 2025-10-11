package grafana

import (
	"grafana-provisioner/config"
	"time"
)

// ClientParams defines parameters required for creating Grafana client
type ClientParams struct {
	URL        string
	Token      string
	Timeout    time.Duration
	Retries    int
	RetryDelay time.Duration
}

// PostgreSQLDataSourceModel defines the JSON structure required by Grafana
// to create a new PostgreSQL data source.
type PostgreSQLDataSourceModel struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // Must be "postgres"
	Access    string `json:"access"`
	URL       string `json:"url"`  // Host:Port, e.g., "127.0.0.1:5432"
	Database  string `json:"database"`
	User      string `json:"user"`
	Password  string `json:"password"`
	SSLMode   string `json:"sslmode"` // e.g., "disable", "require"
	IsDefault bool   `json:"isDefault"`
}

type CreateDataSourceResponseDatasource struct {  
	ID      int    `json:"id"`
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// CreateDataSourceResponse is the structure of the response from the Grafana API
// after successful creation.
type CreateDataSourceResponse struct {
	Datasource  CreateDataSourceResponseDatasource `json:"datasource"`
}

// DashboardImportRequest is the structure for importing a dashboard via API.
type DashboardImportRequest struct {
	Dashboard DashboardJSON `json:"dashboard"`
	Inputs    []interface{} `json:"inputs,omitempty"`
	FolderID  int           `json:"folderId,omitempty"`
	FolderUID string        `json:"folderUid,omitempty"`
	Overwrite bool          `json:"overwrite"`
	Message   string        `json:"message,omitempty"`
	Path      string        `json:"path,omitempty"`
}

// DashboardJSON represents the complete dashboard JSON
type DashboardJSON map[string]interface{}

// Dashboard represents the data for dashboard creation
type Dashboard struct {
	Name       string
	Folder     string
	File       string
	DataSource string
	ImportVar  string
}

// DataSource represents the data for data source creation
type DataSource struct {
	ID         int
	UID        string
	Name       string
	Type       string
	URL        string
	User	   string
	Password   string
	SSLMode    string
	IsDefault  bool
	Database   string
}

// FolderMapping holds the runtime information about a provisioned folder.
type FolderMapping struct {
	ID    int
	UID   string
	Title string
}

// Config defines the configuration subset needed for provisioning
type Config struct {
	Grafana        ClientParams
	Dashboard      Dashboard
	DataSources    []DataSource
	Folders        []config.FolderConfig 
	FoldersMapping map[string]FolderMapping
}

// FolderResponse is the structure for an existing Grafana folder
type FolderResponse struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// CreateFolderRequest is the structure for creating a folder via API.
type CreateFolderRequest struct {
	UID   string `json:"uid,omitempty"` // Optional UID
	Title string `json:"title"`
}

// DashboardSearchResponse является структурой для дашборда, возвращаемого конечной точкой /api/search
type DashboardSearchResponse struct {
	ID          int    `json:"id"`
	UID         string `json:"uid"`
	Title       string `json:"title"`
	URI         string `json:"uri"` 
	Type        string `json:"type"`
	FolderID    int    `json:"folderId"`
	FolderUID   string `json:"folderUid"`
	FolderTitle string `json:"folderTitle"`
}