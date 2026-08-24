# LANventory

LANventory is an actively developed fork of WatchYourLAN focused on modern LAN inventory, device presence history, events and classification.

> This project is currently under active development. Features, configuration and UI may change.

LANventory is based on [WatchYourLAN by aceberg](https://github.com/aceberg/WatchYourLAN). The original scanning and backend foundation comes from the upstream project; this repository contains additional fork work around the interface, offline runtime assets, history, events, retention and device management. Applicable upstream license and attribution notices are preserved.

Current repository: [godlev/WatchYourLAN2](https://github.com/godlev/WatchYourLAN2).

## What's different in LANventory

### Interface

- Modern compact dashboard with responsive layout.
- Dark and light color modes.
- Locally bundled Open Sans.
- Locally bundled Bootstrap Icons.
- Locally bundled Bootswatch themes.
- No automatic external UI asset dependency after build.

### Device management

- Persistent Known/Unknown classification.
- Persistent manual Device Type classification.
- Device Type icon picker.
- Host read mode.
- Explicit Host edit mode.

### Presence

Presence is sampled online/offline visibility over time. The fork includes configurable Presence retention, day/night visualization and hour/day timeline boundaries.

### Events

Events store meaningful transitions and actions rather than every scan sample. Current event types are:

- `discovered`
- `online`
- `offline`
- `known`
- `unknown`
- `device-type-changed`

The UI includes a unified Events explorer, Device/Event filters, summary cards, Group By controls and Home panels split into Connectivity and Device Changes.

### Retention

- Presence samples use Presence retention.
- Online/Offline events use configurable Connectivity event retention.
- Device-change events remain while the device record exists.

## Development Status

LANventory is actively under development and is currently based on the upstream WatchYourLAN architecture. Not all existing upstream documentation may apply exactly to this fork yet. Migrations and backward compatibility are being kept in mind, but the fork should not be treated as a separately stabilized production release until that status is documented here.

## Screenshots

Updated LANventory screenshots will be added as development progresses.

## Support Development

If you find LANventory useful and would like to support development:

[Support via Revolut](https://revolut.me/mirgeo)

## Runtime Requirements

LANventory inherits the upstream network discovery model:

- Linux is the intended runtime for real LAN scanning.
- `arp-scan` is required for real ARP discovery.
- Host networking is typically required in containers so the scanner can see local network interfaces.
- The web UI is served by the Go backend, normally on `http://0.0.0.0:8840` unless configured otherwise.

The Go backend starts the scanner during normal application startup. Use the mock API described below when previewing the frontend without touching the LAN.

## Development Preview

The frontend can be previewed safely with mock data. This mode does not scan the LAN, send notifications, send Wake-on-LAN packets, perform port scans or write persistent production data.

```sh
cd frontend
npm install
npm run mock:api
```

In another terminal:

```sh
cd frontend
npm run dev -- --host 127.0.0.1
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). Vite proxies `/api` and `/fs` to the local mock API at `http://127.0.0.1:8840` during development.

## Building From Source

Build the frontend:

```sh
cd frontend
npm run build
```

Run backend checks:

```sh
cd backend
go test ./...
go vet ./...
go build ./...
```

To run the real backend from source, provide an intentional data/config directory:

```sh
cd backend
go run ./cmd/WatchYourLAN -d /path/to/dev-data
```

The backend creates or reads `config_v2.yaml` and `scan.db` in that directory. Real backend startup begins network scanning, so do this only on a machine where `arp-scan` is installed and LAN scanning is intended.

## Installation Status

LANventory does not currently document an independently published Docker image or packaged release artifact in this repository. Do not install `aceberg/watchyourlan` expecting to receive LANventory fork changes; that image belongs to the upstream project.

The existing Dockerfile and compose files in this repository are inherited development references and may still contain upstream naming. Build and test from this checkout when evaluating fork behavior.

## Configuration

Configuration can be supplied through `config_v2.yaml`, the UI or environment variables. Keys in the config file match the environment variable names in lowercase.

### Basic Config

| Variable | Description | Default |
| --- | --- | --- |
| `TZ` | Time zone for correct timestamps. | |
| `HOST` | Listen address. | `0.0.0.0` |
| `PORT` | Web UI/API port. | `8840` |
| `THEME` | Bundled Bootswatch theme name in lowercase. | `sand` |
| `COLOR` | Color mode: `dark` or `light`. | `dark` |
| `NODEPATH` | Legacy upstream compatibility setting. LANventory bundles UI assets locally. | |
| `SHOUTRRR_URL` | Shoutrrr notification URL. See [Shoutrrr service documentation](https://shoutrrr.nickfedor.com/services/overview/). | |

### Scan And Database Settings

| Variable | Description | Default |
| --- | --- | --- |
| `IFACES` | Interfaces to scan, separated by spaces. See the upstream [VLAN and ARP scan guide](https://github.com/aceberg/WatchYourLAN/blob/main/docs/VLAN_ARP_SCAN.md). | |
| `TIMEOUT` | Time between scans in seconds. | `120` |
| `ARP_ARGS` | Additional arguments passed to `arp-scan`. | |
| `ARP_STRS`, `ARP_STRS_JOINED` | Optional ARP result strings. See the upstream [VLAN and ARP scan guide](https://github.com/aceberg/WatchYourLAN/blob/main/docs/VLAN_ARP_SCAN.md). | |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn` or `error`. | `info` |
| `USE_DB` | Database backend: `sqlite` or `postgres`. | `sqlite` |
| `PG_CONNECT` | PostgreSQL connection string. Parameters are documented by [lib/pq](https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters). | |

### Retention Settings

| Variable | Description | Default |
| --- | --- | --- |
| `TRIM_HIST` | Presence sample retention in hours. Used by the Presence page. | `48` |
| `CONNECTIVITY_RETENTION` | Online/Offline Event retention in hours. When absent from an older config, the current implementation falls back to `TRIM_HIST`. | `TRIM_HIST` |
| `HIST_IN_DB` | Deprecated upstream setting. History is stored in the database. | |

### InfluxDB2

| Variable | Description | Default |
| --- | --- | --- |
| `INFLUX_ENABLE` | Enable export to InfluxDB2. | `false` |
| `INFLUX_SKIP_TLS` | Skip TLS verification. | `false` |
| `INFLUX_ADDR` | InfluxDB2 server URL. | |
| `INFLUX_BUCKET` | InfluxDB2 bucket. | |
| `INFLUX_ORG` | InfluxDB2 organization. | |
| `INFLUX_TOKEN` | InfluxDB2 token. | |

### Prometheus

| Variable | Description | Default |
| --- | --- | --- |
| `PROMETHEUS_ENABLE` | Enable the `/metrics` endpoint. | `false` |

## Offline UI Assets

The built LANventory UI is intended to be self-contained. Bootstrap Icons, Open Sans and Bootswatch themes are installed during development/build and bundled or copied into the application assets served by LANventory itself.

The browser should not need Internet access to render the UI. User-clicked links to external repositories, documentation, Shoutrrr, Bootswatch, package documentation or donation pages remain normal external navigation.

## API And Integrations

- API notes: [docs/API.md](docs/API.md)
- Prometheus: enable `PROMETHEUS_ENABLE` and read metrics from `/metrics`.
- InfluxDB2: configure the InfluxDB settings above.

Some integrations and packaging references may still live in upstream documentation. Treat those as upstream references unless they are updated in this fork.

## Auth And Exposure

LANventory does not currently provide built-in authentication and should be treated as a trusted-network tool. Do not expose the UI/API directly to the public Internet. If remote access is required, place it behind strong authentication, TLS and network controls such as a VPN or authenticated reverse proxy.

Stored notification, PostgreSQL and InfluxDB secrets are write-only in the Settings UI and are redacted from `/api/config`, but the local config file remains sensitive. Protect the runtime config/data directory with normal host filesystem permissions and avoid sharing logs or config backups without reviewing them first.

Real scanning typically needs host network access and `arp-scan`, so only run the real backend on systems where LAN discovery is intentional. Use the mock API for frontend preview work that should not touch the network.

## Thanks And Attribution

- Based on [WatchYourLAN by aceberg](https://github.com/aceberg/WatchYourLAN).
- Favicon and logo: [Access point icons created by Freepik - Flaticon](https://www.flaticon.com/free-icons/access-point).
- [Bootstrap](https://getbootstrap.com/).
- [Bootswatch](https://bootswatch.com/).
- [Bootstrap Icons](https://icons.getbootstrap.com/).
- Go and JavaScript package authors listed in the project dependency manifests.
