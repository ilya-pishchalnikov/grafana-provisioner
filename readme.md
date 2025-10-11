# 🚀 grafana-provisioner

A lightweight, opinionated Go application designed to provision Grafana infrastructure (Data Sources, Folders, and Dashboards) using the Grafana HTTP API. It runs as a containerized bootstrap service to ensure essential monitoring components are configured before application services start.

## 🌟 Overview

The `grafana-provisioner` coordinates a complete provisioning workflow, ensuring the target Grafana instance is ready for use.

Key provisioning steps include:

1.  **Grafana API Wait:** The application waits for the Grafana API (`/api/health`) to become available, handling retries automatically.
2.  **Data Source Provisioning (PostgreSQL):**
    * Creates **PostgreSQL data sources** based on the `datasources` configuration.
    * Implements logic to **skip creation** if a source with the same type, URL, and database already exists.
    * Resolves **name conflicts** for new data sources by appending a counter (`_1`, `_2`, etc.).
3.  **Folder Provisioning:** Creates all Grafana folders defined in the `folders` configuration section.
4.  **Dashboard Provisioning:**
    * Imports **multiple dashboards** from local JSON files.
    * **Overwrites** existing dashboards to guarantee the latest version from the file is applied.
    * **Supports Multiple Data Source Imports:** Resolves the UIDs for all configured data sources and injects them into the respective dashboard variables specified in the `imports` array.

---

## ⚙️ Configuration

The application is configured via the **`config.yaml`** file. All configuration values support **environment variable expansion** (e.g., `${GF_ADMIN_TOKEN}`).

### `config.yaml` Structure

| Section | Key | Type | Description | Required |
| :--- | :--- | :--- | :--- | :--- |
| **log** | `level` | `string` | Minimum logging level (`debug`, `info`, `warn`, `error`). | Yes |
| | `format` | `string` | Log output format (`json`, `text`). | Yes |
| **grafana** | `url` | `string` | Base URL of the Grafana instance (e.g., `http://grafana:3000`). | Yes |
| | `token` | `string` | Grafana Admin or Service Account API Token. | Yes |
| | `timeout` | `duration` | HTTP client timeout (e.g., `30s`). | No (Default: `30s`) |
| | `retries` | `int` | Number of retries for API availability check. | No (Default: `5`) |
| | `retry-delay` | `duration` | Delay between API availability retries (e.g., `10s`). | No (Default: `10s`) |
| **folders** | `name` | `string` | List of folder names to be created in Grafana. | No |
| **datasources** | `name` | `string` | Unique internal name for the data source. | Yes |
| | `host` | `string` | PostgreSQL host. | Yes |
| | `port` | `int` | PostgreSQL port (e.g., `5432`). | Yes |
| | `user`, `password` | `string` | PostgreSQL credentials. | Yes |
| | `dbname` | `string` | PostgreSQL database name. | Yes |
| | `sslmode` | `string` | PostgreSQL SSL mode (e.g., `disable`, `require`). | Yes |
| **dashboards** | `name` | `string` | Display name of the dashboard in Grafana. | Yes |
| | `file` | `string` | Path to the local dashboard JSON file (e.g., `"assets/dashboard.json"`). | Yes |
| | `folder` | `string` | Target Grafana folder name. Must be defined in `folders` or be `"General"`. | Yes |
| | **`imports`** | `array` | **List of data source mappings (key change).** | Yes |
| | `imports[*].name` | `string` | The dashboard variable name (e.g., `DS_PROMETHEUS`) to be replaced. | Yes |
| | `imports[*].datasource` | `string` | The **name** of the data source from the `datasources` section to link. | Yes |

### Example `config.yaml`

This example demonstrates the new `imports` structure for linking multiple data sources to a single dashboard.

```yaml
# Configuration for grafana-provisioner

log:
    level: info # debug, info, warn, error
    format: text # json, text

grafana:
    url: http://elmon-grafana:3000
    token: ${GF_ADMIN_TOKEN}
    timeout: 30s
    retries: 5
    retry-delay: 10s

folders:
    - name: elmon
    - name: logs_and_metrics

datasources:
    -
        name: metrics_pg_db
        host: metrics-postgres
        port: 5432
        user: collector
        password: ${METRICS_DB_PASSWORD}
        dbname: metrics
        sslmode: disable
    -
        name: logs_pg_db
        host: logs-postgres
        port: 5432
        user: collector
        password: ${LOGS_DB_PASSWORD}
        dbname: logs
        sslmode: disable

dashboards:
    - 
        name: Multi-Source Dashboard
        file: "assets/multi_source_dashboard.json"
        folder: logs_and_metrics
        imports: 
            -
                name: DS_METRICS_VAR  # Variable in dashboard
                datasource: metrics_pg_db # Data source defined above
            -
                name: DS_LOGS_VAR     # Another variable in dashboard
                datasource: logs_pg_db    # Second data source defined above
````

-----

## 📦 Building and Running

The application uses a multi-stage `Dockerfile` to produce a minimal production image based on Alpine.

### Build the Container

```bash
docker build -t grafana-provisioner:latest .
```

### Run with Docker

To execute the provisioner, you must mount the configuration and dashboard assets, and provide all necessary secrets as environment variables.

```bash
docker run --rm \
    --network my_stack_network \
    -e GF_ADMIN_TOKEN=your_grafana_token \
    -e METRICS_DB_PASSWORD=your_metrics_secret \
    -v /path/to/your/config.yaml:/app/config.yaml \
    -v /path/to/your/assets:/app/assets \
    grafana-provisioner:latest
```

### Docker Compose Integration

The provided `docker-compose.yml` demonstrates how to integrate the provisioner as a service in a larger stack.

```yaml
# Snippet from docker-compose.yml

services:
  grafana-provisioner:
      container_name: grafana-provisioner
      build:
          context: .
          dockerfile: Dockerfile
      environment:
          CONFIG_PATH: config.yaml
          GF_ADMIN_TOKEN: ${GF_ADMIN_TOKEN}
          METRICS_DB_PASSWORD: ${METRICS_DB_PASSWORD}
          # ... (Add any other required secrets here)
      volumes:
          - ./config.yaml:/app/config.yaml:ro
          - ./assets:/app/assets:ro
      depends_on:
          # Ensure Grafana and the Database are up before provisioning starts
          - grafana 
          - postgres-metrics
      restart: "no" # Provisioning should only run once
```

-----

## 🛠️ Development Setup

A fully configured **Visual Studio Code Dev Container** setup is provided for a consistent, self-contained Go development environment.

### Prerequisites

  * Docker
  * Visual Studio Code
  * VS Code Remote - Containers extension

### Dev Container Configuration

The setup uses `dev.Dockerfile` and `devcontainer.json`:

  * **Image:** Built from `dev.Dockerfile` (based on `golang:1.24-alpine`).
  * **Tools:** Includes the Delve debugger (`dlv`).
  * **Networking:** The `devcontainer.json` includes `runArgs` to add necessary security options for debugging (`--cap-add=SYS_PTRACE`, `--security-opt=seccomp=unconfined`) and to connect the container to a specific Docker network (`--net=elmon_elmon_network` in the example) for interaction with services like Grafana or PostgreSQL.

To start development:

1.  Open the project folder in VS Code.
2.  VS Code will prompt you to "Reopen in Container".
3.  Once the container is built and running, you can set breakpoints and use the Go debugger tools.

-----

## 📄 License

This project is licensed under the **MIT License**. See the `LICENSE` file for details.

```
```