<p align="center">
  <img src="assets/lanventory-128x128.png" width="96" alt="LANnventory icon" />
</p>

<h1 align="center">LANnventory</h1>

<p align="center">
  Modern local-first LAN inventory, presence monitoring and device event tracking.
</p>

LANnventory is an independently developed project based on the original [WatchYourLAN by aceberg](https://github.com/aceberg/WatchYourLAN). It keeps the lightweight ARP-scanning foundation while expanding the project with a modern interface, persistent device classification, Presence history, Events, configurable retention, safer configuration handling and release-readiness hardening.

> [!IMPORTANT]
> LANnventory is still under active development. Packaging, installation and upgrade-path validation are still being completed before the first beta release.

Current repository: [godlev/WatchYourLAN2](https://github.com/godlev/LANnventory)

## Dashboard

![LANnventory dashboard](assets/image.png)

## Highlights

### Modern local-first interface

- Compact responsive dashboard.
- Dark and light modes.
- Local Open Sans fonts.
- Local Bootstrap Icons.
- Local Bootswatch themes.
- Dedicated LANnventory favicon and navbar icons.
- No automatic external UI asset requests after build.

### Home dashboard

- Summary cards for Total, Online, Offline, Known and Unknown devices.
- Filtering by interface, Device Type, Known/Unknown state and Online/Offline state.
- Search across device information such as name, IP, MAC and hardware.
- Device Type filtering integrated with dashboard counts and recent events.
- Compact device table with sticky headings.
- Device names link to Host details.
- Direct Host edit shortcut.
- Separate recent **Connectivity** and **Device Changes** panels.
- Recent-event panels automatically follow the currently filtered device set.

### Device management

- Persistent Known / Unknown classification.
- Persistent manual Device Type classification.
- Device Type icon picker.
- Host page defaults to read mode.
- Explicit Host edit mode.
- Wake-on-LAN and port-scan actions remain available where supported.

Supported Device Types currently include:

`Router`, `Switch`, `Access Point`, `Firewall`, `Server`, `NAS`, `Desktop`, `Laptop`, `Phone`, `Tablet`, `TV`, `Printer`, `Camera`, `IoT`, `Virtual Machine`, `Container`, `Game Console`, and `Other`.

## Presence

Presence represents sampled online/offline visibility over time.

Features include:

- Search by device name, IP and other shared device fields.
- Interface filtering.
- Device Type filtering.
- Known / Unknown filtering.
- Current Online / Offline filtering.
- Day/night visualization.
- Hour and day timeline boundaries.
- Configurable Presence retention.
- Presence data remains separate from discrete Events.

## Events

Events record meaningful state transitions and device changes instead of treating every scan sample as an event.

Current Event types:

- `discovered`
- `online`
- `offline`
- `known`
- `unknown`
- `device-type-changed`

Events features include:

- Unified Events explorer.
- Summary-card filtering.
- Ctrl/Cmd-click multi-select for Event summary cards.
- Device filtering.
- Multiple Event Type selection.
- Active filter/grouping indicators.
- Sticky Events table header.
- Deterministic pagination.
- Load More support.
- Group By controls.
- Collapse All / Expand All.
- Stable collapsed state while loading additional Events.

Available Group By options include:

- Device
- Event
- Category
- Device Type
- IP
- Interface
- Day

### Event device display

The Device column can be displayed as:

- `Name + icon`
- `Name only`
- `Icon only`

`Icon only` is the default for new or invalid preferences.

When a Host still exists, the device icon/name can link to its Host page. If a Host has been deleted, retained connectivity Events remain readable historical snapshots and do not link to an unrelated Host that later reuses the same numeric ID.

## Retention model

LANnventory deliberately separates sampled Presence history from Events.

| Data | Retention behavior |
| --- | --- |
| Presence samples | Controlled by `TRIM_HIST` |
| `online` / `offline` Events | Controlled by `CONNECTIVITY_RETENTION` |
| Device-change Events | Retained while the device record exists |

If `CONNECTIVITY_RETENTION` is absent from an older configuration, LANnventory falls back to the existing `TRIM_HIST` value for backward compatibility.

Retention can be configured from **Settings → Data retention** without restarting the network scanner.

## Reliability and migration work

LANnventory has completed a major release-readiness audit and hardening pass.

Current validation includes:

- Non-destructive migration tests against a legacy WatchYourLAN SQLite schema.
- Existing Host and History rows survive migration.
- Device Type and Events schema additions are migration-tested.
- Migration is idempotent.
- Reopening an already migrated database is safe.
- Failed scans do not create false Offline state changes.
- Failed scans do not create false Offline Events.
- Repeated scans do not create duplicate transition Events.
- Host return after a failed scan produces the correct transition.
- Presence and connectivity-event retention are independent.
- Device-change Events are not removed by age-based connectivity retention.
- Missing Host IDs are rejected instead of producing zero-value Hosts.
- Configuration validation is atomic.
- Invalid configuration changes do not partially mutate runtime configuration.
- Invalid scan settings do not accidentally restart scanning.
- Database-setting changes trigger schema migration after reconnect.
- Runtime configuration access is synchronized for concurrent readers/writers.
- Global database lifecycle and reconnect operations are mutex-guarded.

## Configuration security hardening

Sensitive configuration values are no longer returned by `/api/config`.

Sensitive settings include values such as:

- PostgreSQL connection credentials.
- InfluxDB token.
- Shoutrrr notification URL/credentials.

These values are write-only from the web Settings interface.

Existing secret values can be:

- kept unchanged,
- replaced,
- explicitly cleared.

Leaving a secret field blank does **not** overwrite the stored value. The frontend only receives whether a secret is configured, not the stored plaintext secret itself.

## Frontend failure handling

Critical UI flows have been hardened so API failures do not appear as legitimate empty/default data.

Examples include:

- Missing Host handling.
- Settings load failures.
- Settings save failures.
- Host API failures.
- Presence API failures.
- Events API failures.

The Settings page no longer displays fake/default configuration values as though they were the current stored configuration after a load failure.

## Security and exposure

LANnventory does **not** currently provide built-in authentication.

> [!WARNING]
> Do not expose LANnventory directly to the public Internet.

Recommended deployment:

- trusted local network,
- VPN,
- authenticated reverse proxy,
- SSO-protected reverse proxy.

LANnventory exposes administrative operations including device editing/deletion, scan configuration, runtime configuration, Wake-on-LAN and port scanning. Protect access accordingly.

Stored PostgreSQL, InfluxDB and Shoutrrr secrets are protected from browser readback, but the local `config_v2.yaml` file still contains the real values. Protect the LANnventory data/config directory using appropriate filesystem permissions.

## Runtime requirements

LANnventory currently uses the WatchYourLAN ARP discovery model.

Real LAN scanning is intended primarily for Linux.

Requirements include:

- `arp-scan`
- `tzdata`
- access to the physical LAN interface being scanned.

The web UI/API normally listens on:

```text
0.0.0.0:8840
```

unless configured otherwise.

### Container networking

Real ARP-based LAN discovery generally requires access to the host network interfaces. For container deployments, host networking is typically required. Docker bridge networking may expose only the container network rather than the physical LAN.

Container packaging and network-permission behavior are still being formally validated before the first beta.

## Safe development preview

The frontend includes a localhost-only mock API for UI development.

The mock environment does **not**:

- run `arp-scan`,
- scan the LAN,
- send Wake-on-LAN packets,
- run real port scans,
- send notifications,
- connect to production InfluxDB,
- connect to production PostgreSQL,
- modify production LANnventory data.

Install frontend dependencies:

```sh
cd frontend
npm install
```

Start the mock API:

```sh
npm run mock:api
```

In another terminal:

```sh
cd frontend
npm run dev:local
```

Open:

```text
http://127.0.0.1:5173
```

Vite proxies `/api` and `/fs` to the local mock API on `127.0.0.1:8840` during development.

## Build from source

### Frontend

```sh
cd frontend
npm install
npm run build
npm audit
npm audit --omit=dev
```

### Backend

```sh
cd backend
go test ./...
go vet ./...
go build ./...
```

To run the real backend from source, provide an intentional development data directory:

```sh
cd backend
go run ./cmd/WatchYourLAN -d /path/to/dev-data
```

The command path is still inherited for compatibility at the current development stage. Real backend startup begins network scanning, so only run it where LAN scanning is intended and `arp-scan` is available.

## Installation status

LANnventory does not yet advertise a separately published stable Docker image or release package.

> [!CAUTION]
> Do not install `aceberg/watchyourlan` expecting LANnventory functionality. That image belongs to the upstream WatchYourLAN project.

Docker/container packaging, persistent-volume mapping, clean-install validation and the operational WatchYourLAN → LANnventory upgrade path are being prepared before the first beta release.

Until that work is completed, LANnventory should be evaluated by building directly from this repository.

## Configuration

Configuration can be supplied through:

- `config_v2.yaml`
- Web Settings
- environment variables

Config-file keys use lowercase equivalents of the environment variable names.

### General

| Variable | Description | Default |
| --- | --- | --- |
| `TZ` | Time zone used for timestamps. | |
| `HOST` | Listen address. | `0.0.0.0` |
| `PORT` | Web UI/API port. | `8840` |
| `THEME` | Bundled Bootswatch theme name. | `sand` |
| `COLOR` | Color mode: `dark` or `light`. | `dark` |
| `NODEPATH` | Legacy compatibility setting. LANnventory bundles UI assets locally. | |
| `SHOUTRRR_URL` | Shoutrrr notification URL. | |

Shoutrrr documentation: [shoutrrr.nickfedor.com](https://shoutrrr.nickfedor.com/services/overview/)

### Scanning and database

| Variable | Description | Default |
| --- | --- | --- |
| `IFACES` | Interfaces to scan, separated by spaces. | |
| `TIMEOUT` | Time between scans in seconds. | `120` |
| `ARP_ARGS` | Additional arguments passed to `arp-scan`. | |
| `ARP_STRS` | Optional ARP result configuration inherited from upstream. | |
| `ARP_STRS_JOINED` | Optional ARP result configuration inherited from upstream. | |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. | `info` |
| `USE_DB` | Database backend: `sqlite` or `postgres`. | `sqlite` |
| `PG_CONNECT` | PostgreSQL connection string. | |

Upstream VLAN / ARP scan documentation: [VLAN_ARP_SCAN.md](https://github.com/aceberg/WatchYourLAN/blob/main/docs/VLAN_ARP_SCAN.md)

PostgreSQL connection parameters: [lib/pq](https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters)

### Data retention

| Variable | Description | Default |
| --- | --- | --- |
| `TRIM_HIST` | Presence sample retention in hours. | `48` |
| `CONNECTIVITY_RETENTION` | Online / Offline Event retention in hours. | `TRIM_HIST` |
| `HIST_IN_DB` | Deprecated compatibility setting. History is always stored in the database. | |

### InfluxDB2

| Variable | Description | Default |
| --- | --- | --- |
| `INFLUX_ENABLE` | Enable export to InfluxDB2. | `false` |
| `INFLUX_SKIP_TLS` | Skip TLS certificate verification. | `false` |
| `INFLUX_ADDR` | InfluxDB2 server URL. | |
| `INFLUX_BUCKET` | InfluxDB2 bucket. | |
| `INFLUX_ORG` | InfluxDB2 organization. | |
| `INFLUX_TOKEN` | InfluxDB2 token. Stored value is write-only in the UI. | |

### Prometheus

| Variable | Description | Default |
| --- | --- | --- |
| `PROMETHEUS_ENABLE` | Enable the `/metrics` endpoint. | `false` |

## Local / offline UI assets

LANnventory's built web interface is self-contained.

Runtime UI assets are local, including:

- Bootstrap Icons,
- Open Sans,
- Bootswatch themes,
- LANnventory favicon assets,
- LANnventory navbar icon,
- JavaScript bundles,
- CSS bundles.

The repository's current branding/icon source set is stored under [`assets/`](assets/), including small favicon sizes and high-resolution artwork.

No CDN, Google Fonts or other automatic external UI asset request is required for rendering the LANnventory interface. User-clicked external links remain normal external navigation.

## Branding assets

Current source assets include:

- [`assets/favicon.png`](assets/favicon.png)
- [`assets/lanventory-16x16.png`](assets/lanventory-16x16.png)
- [`assets/lanventory-32x32.png`](assets/lanventory-32x32.png)
- [`assets/lanventory-48x48.png`](assets/lanventory-48x48.png)
- [`assets/lanventory-64x64.png`](assets/lanventory-64x64.png)
- [`assets/lanventory-128x128.png`](assets/lanventory-128x128.png)
- [`assets/lanventory-180x180.png`](assets/lanventory-180x180.png)
- [`assets/lanventory-192x192.png`](assets/lanventory-192x192.png)
- [`assets/lanventory-256x256.png`](assets/lanventory-256x256.png)
- [`assets/lanventory-512x512.png`](assets/lanventory-512x512.png)
- [`assets/lanventory-1254-1254.png`](assets/lanventory-1254-1254.png)
- [`assets/lanventory-navbar.png`](assets/lanventory-navbar.png)
- [`assets/lanventory.ico`](assets/lanventory.ico)

The product name is **LANnventory**. Asset filenames currently retain the existing `lanventory-*` filename prefix.

## API and integrations

- API documentation: [docs/API.md](docs/API.md)
- Prometheus: enable `PROMETHEUS_ENABLE` and use `/metrics`.
- InfluxDB2 remains supported through the configuration options above.
- Swagger metadata is branded for LANnventory while compatibility-sensitive API/module paths remain intact for now.

## PostgreSQL status

LANnventory retains PostgreSQL support and fork-added database operations have been reviewed for PostgreSQL/GORM portability.

> [!NOTE]
> A real isolated PostgreSQL runtime integration test is still required before the first beta is declared fully validated for PostgreSQL deployments.

SQLite is currently the most extensively tested migration and runtime path.

## Current validation status

Recent development validation has included:

```text
npm audit
npm audit --omit=dev
npm run build
node --check frontend/dev/mock-api.mjs

go test ./...
go vet ./...
go build ./...
```

Recent dependency audits reported **0 vulnerabilities**.

The Go race detector has not been run in the current Windows development environment because CGO is disabled and gcc is not installed there. No toolchain was installed solely to enable race testing.

## Current development focus

Major completed work includes:

- UI modernization.
- Responsive dashboard.
- Dark/light themes.
- Fully local runtime UI assets.
- LANnventory branding and icon set.
- Device Type persistence and filtering.
- Persistent Events.
- Presence / Events separation.
- Event retention controls.
- Event grouping and multi-filtering.
- Event Device display preferences.
- Deleted-host Event snapshot safety.
- Settings redesign.
- API validation hardening.
- Legacy SQLite migration testing.
- Scanner failure/Event semantics testing.
- Configuration atomicity.
- Sensitive config write-only handling.
- Configuration concurrency hardening.
- Database lifecycle hardening.
- Critical frontend failure-state hardening.

Current focus: **Docker / packaging / installation / upgrade-path validation before the first beta release.**

## Support development

If you find LANnventory useful and would like to support continued development:

[Support LANnventory via Revolut](https://revolut.me/mirgeo)

## Upstream project and attribution

LANnventory is independently developed from a codebase originally based on [WatchYourLAN by aceberg](https://github.com/aceberg/WatchYourLAN).

The project preserves applicable upstream attribution and licensing notices while evolving independently.

Thanks to:

- [WatchYourLAN by aceberg](https://github.com/aceberg/WatchYourLAN)
- [Bootstrap](https://getbootstrap.com/)
- [Bootswatch](https://bootswatch.com/)
- [Bootstrap Icons](https://icons.getbootstrap.com/)
- Open Sans / Fontsource
- Go package authors
- JavaScript package authors

## Repository naming

The product is branded **LANnventory**.

The GitHub repository currently remains:

```text
godlev/WatchYourLAN2
```

The Go module currently remains:

```text
github.com/aceberg/WatchYourLAN
```

Those names are scheduled for a dedicated independence/rename phase after packaging validation, so data/runtime compatibility changes can be reviewed separately.
