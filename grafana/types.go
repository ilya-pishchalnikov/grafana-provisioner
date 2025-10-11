package grafana

import (
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

// DataSource defines the parameters for a data source provisioned by this tool.
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

// DashboardImport defines a single variable mapping for data source injection.
type DashboardImport struct {
	Name       string // The name of the dashboard variable (e.g., DS_ELMON_METRICS)
	DataSource string // The name of the data source from the config (e.g., elmon_metrics)
}

// Dashboard defines parameters of a Grafana dashboard.
type Dashboard struct {
	Name       string 
	Folder     string 
	File       string 
	DataSource string 
	ImportVar  string
	Imports    []DashboardImport 
}

// Folder defines parameters of a Grafana folder from config.
// NOTE: This structure was moved from the config package to decouple grafana package.
type Folder struct {
	Name string 
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
	Dashboards     []Dashboard
	DataSources    []DataSource
	Folders        []Folder
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

// DashboardSearchResponse is the structure for a dashboard returned by the /api/search endpoint
type DashboardSearchResponse struct {
	ID          int    `json:"id"`
	UID         string `json:"uid"`
	Title       string `json:"title"`
	URI         string `json:"uri"`
	URL         string `json:"url"`
	Slug        string `json:"slug"`
	Type        string `json:"type"`       // e.g. "dash-db"
	Tags        []string `json:"tags"`
	IsStarred   bool   `json:"isStarred"`
	FolderID    int    `json:"folderId"`
	FolderUID   string `json:"folderUid"`
	FolderTitle string `json:"folderTitle"`
	FolderURL   string `json:"folderUrl"`
}

// DashboardJSON is a type alias for a raw dashboard JSON map.
type DashboardJSON map[string]interface{}

// DashboardImportRequest is the structure for importing a dashboard via API.
type DashboardImportRequest struct {
	Dashboard DashboardJSON `json:"dashboard"`
	Inputs    []interface{} `json:"inputs"`
	FolderUID string        `json:"folderUid"`
	Overwrite bool          `json:"overwrite"`
	Message   string        `json:"message"`
}