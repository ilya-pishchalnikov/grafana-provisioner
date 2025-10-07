# grafana-provisioner

A lightweight, opinionated Go application designed to provision a PostgreSQL data source and import a Grafana dashboard from a JSON file using the Grafana HTTP API.

## 🚀 Overview

The `grafana-provisioner` ensures that a Grafana instance is set up with the necessary data source and dashboard before application services start. It handles:

1.  Waiting for the Grafana API to become available.
2.  Creating a **PostgreSQL data source**, with logic to skip creation if a data source with the same type, URL, and database already exists. It also handles name conflicts by appending a counter (`_1`, `_2`, etc.) to the configured name.
3.  **Importing a dashboard** from a local JSON file, overwriting any existing one to ensure the latest version is always in place.
4.  Dynamically injecting the UID of the newly created or existing data source into the dashboard's import variables.

## ⚙️ Configuration

The application is configured primarily via the `config.yaml` file and environment variables for secrets.

### `config.yaml` Structure

The default configuration is loaded from a file like `config.yaml`. Environment variables (e.g., `${GF_ADMIN_TOKEN}`) are expanded at runtime.

| Section | Key | Type | Description | |
| :--- | :--- | :--- | :--- | :--- |
| **log** | `level` | `string` | Minimum logging level (`debug`, `info`, `warn`, `error`). | |
| | `format` | `string` | Log output format (`json`, `text`). | |
| --- |
| **grafana** | `url` | `string` | Base URL of the Grafana instance. |  |
| | `token` | `string` | Grafana Admin API Token (prefers environment variable substitution). **Required**. |  |
| | `timeout` | `Duration` | HTTP client timeout for API requests (e.g., `30s`, `1m`). |  |
| | `retries` | `int` | Number of times to retry failed API calls (including the initial wait for API readiness). | |
| | `retry-delay` | `Duration` | Delay between retry attempts. |  |
| | **dashboard** | `name` | `string` | The desired name for the imported dashboard. | |
| | | `file` | `string` | The path to the local dashboard JSON file to import. |  |
| | | `import-var`| `string` | The name of the input variable in the dashboard JSON (e.g., `__inputs` section) that should be replaced with the provisioned data source's UID. |  |
| | **datasource** | `name` | `string` | The base name for the PostgreSQL data source. Name conflicts are resolved by appending a counter (e.g., `elmon_metrics_1`). |  |
| --- |
| **metrics-db** | `host` | `string` | PostgreSQL host for the data source connection. |  |
| | `port` | `int` | PostgreSQL port. | |
| | `user` | `string` | PostgreSQL user. |  |
| | `password` | `string` | PostgreSQL password (prefers environment variable substitution). | |
| | `dbname` | `string` | PostgreSQL database name. |  |
| | `sslmode` | `string` | PostgreSQL SSL mode (`disable`, `require`, etc.). | |

### Environment Variables

Secrets like the Grafana token and database password **must** be provided via environment variables, as indicated by the `${VAR_NAME}` syntax in `config.yaml`.

* **`GF_ADMIN_TOKEN`**: The authentication token for the Grafana API.
* **`METRICS_DB_PASSWORD`**: The password for the PostgreSQL data source user.

The configuration loading logic uses `os.ExpandEnv` to replace these placeholders before parsing.

## 🛠️ Data Source Provisioning Logic

The application follows a specific process when provisioning the data source:

1.  **Check for Existing Data Source:** It queries Grafana for all existing data sources.
2.  **Match by Connection:** It checks if a data source already exists with the same **`type`** (`grafana-postgresql-datasource`), **`URL`**, and **`Database`**.
    * If a match is found, provisioning is skipped for the data source, and the existing source's UID is used for the dashboard import.
3.  **Resolve Name Conflict:** If the connection is unique, it checks for a name conflict with the configured `grafana.datasource.name`.
    * If a conflict is found, the name is incremented (e.g., `elmon_metrics_1`, `elmon_metrics_2`) until a unique name is found.
4.  **Creation:** The data source is created using the resolved name. Note that the API request includes specific PostgreSQL connection details like `sslmode`, `postgresVersion`, and a secure password field, as seen in `client.go`.
5.  **API Conflict Handling:** If the Grafana API returns a `409 Conflict` (which can happen if a data source with the exact same name was created concurrently), the provisioner treats this as a success and continues.

## 🖼️ Dashboard Provisioning Logic

The dashboard import process is designed for reliability and dependency injection:

1.  **Read File:** The dashboard JSON is read from the path specified in `grafana.dashboard.file`.
2.  **Prepare Import Variables:** The application looks for the `__inputs` section within the raw dashboard JSON.
3.  **Inject Data Source UID:** The `value` for the input variable named by `grafana.dashboard.import-var` (e.g., `DS_ELMON_METRICS`) is set to the **UID** of the provisioned PostgreSQL data source.
4.  **Import:** The dashboard is imported with the `overwrite: true` flag set, guaranteeing that the latest version from the file is always applied.

## 📦 Building and Running

The application is built upon a standard Go environment (as defined in `dev.Dockerfile`):

```dockerfile
FROM golang:1.24-alpine
# ... installs git, bash, curl, gcc, musl-dev
RUN go install [github.com/go-delve/delve/cmd/dlv@v1.24.0](https://github.com/go-delve/delve/cmd/dlv@v1.24.0)
WORKDIR /workspace
````

### Development Environment

A Visual Studio Code Dev Container setup is available via `devcontainer.json`, configured for Go development:

  * **Image:** Built from `dev.Dockerfile` (based on `golang:1.24-alpine`).
  * **Extensions:** `golang.Go`.
  * **Debugging:** Includes `dlv` (Delve) for debugging.
  * **Container Arguments:** Uses specific `runArgs` for capabilities and networking, such as:
      * `--cap-add=SYS_PTRACE`, `--security-opt=seccomp=unconfined` (required for Delve debugger).
      * `--name grafana-go`
      * `--net=elmon_elmon_network` (for network access to other services like Grafana/PostgreSQL).

### Execution

The main function executes the workflow:

1.  Load configuration from `config.yaml` (with ENV expansion).
2.  Initialize the structured logger (`log/slog`).
3.  Construct the internal provisioning `Config` struct.
4.  Call `grafana.RunProvisioning(provisionerConfig, log)`.

Any fatal error during configuration loading or provisioning will cause the application to exit with a non-zero status code.

