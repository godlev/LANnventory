# Change Log
All notable changes to this project will be documented in this file.

## LANnventory Releases

## [v0.1.0-beta.6] - 2026-09-18

### Added
- Persistent Services inventory per device address, protocol and port, including first/last detection, last check and open/closed state.
- Opt-in per-device scheduled TCP service scanning with configurable ports and intervals.
- IPv4/IPv6-aware service attribution through the retained Host identity/address model.
- Conservative service hints for common TCP ports.

### Changed
- Host identity UI now separates managed inventory from discovered network identity evidence and retained address history.
- Manual TCP port scans persist definitive service observations and emit service state Events only on real transitions.
- Scheduled service scans use bounded concurrency, cancellation-safe execution and stale-result protection.
- Host Recent Events show historical IP and MAC snapshots for each event.

### Fixed
- Failed, timed-out, unreachable or cancelled service probes no longer fabricate service-closed state.
- Phase 33 identity/address and Phase 32 scanner/diagnostics regressions remain covered during release validation.

### Migration
- Services and scheduled-scan settings use additive migrations and preserve existing WatchYourLAN/LANnventory data.

## [v0.1.0-beta.3.1] - 2026-09-16

### Added
- Separate background **automatic update checks** from **automatic installation**, allowing notification-only update monitoring.
- Navbar update reminder with the available version when a background or manual check finds a newer release.
- In-app release-notes summary dialog with a separate link to the full GitHub release.
- Curated release-summary loading from the versioned `docs/releases/<tag>.md` file in the published tag.

### Changed
- Navbar update polling now reads cached local status only and no longer causes implicit GitHub release checks.
- Automatic installation still implies automatic checks for backward-compatible beta.3 behavior.

## [v0.1.0-beta.3] - 2026-09-16

### Added
- Redesigned Home and Events summary cards with more compact responsive presentation.
- Current-Unknown device scope in Events.
- Adaptive action tooltips with touch long-press behavior and native browser tooltip fallback.
- Open-port activity events so newly discovered open ports are recorded in Host activity.
- Stable/Beta update channel selection in Settings.
- One-click update status, check and install controls backed by verified GitHub release packages.
- Optional automatic updates using the selected Stable/Beta channel and configurable 6-hour, 12-hour, 24-hour or 7-day check intervals.
- Last-checked timestamp and direct release-notes link in Settings → Updates.
- Versioned pre-update recovery backups for the current LANnventory binary and /etc/watchyourlan data.
- Post-update service and health validation using the configured LANnventory bind address.

### Changed
- Refined Host details and Port Scan layout across desktop breakpoints.
- Improved Host action placement and tooltip behavior.
- Hardened update installation with SHA256 verification, Debian package/architecture checks and recovery-path reporting.
- Release packaging now rebuilds and verifies the embedded frontend before dry runs and published binaries.
- Debian/RPM packages explicitly include curl and CA certificates required by the updater health path.

### Fixed
- Avoided stale embedded frontend assets being packaged into DEB/RPM/APK releases.
- Update health checks no longer assume LANnventory is bound to 127.0.0.1.
- Updater failure handling preserves recovery files and restarts the service when an update cannot complete.

## [v0.1.0-beta.2] - 2026-09-04
### Added
- Event Type and Device multi-select filters for the Events explorer.
- Ordered multi-level Events grouping by device, event, category, device type, IP, interface and day.
- Stable cursor-based Events pagination using event date and ID.
- Additive `DateUTC` event display timestamp for timezone-safe relative event times.
- Official Proxmox VE Debian 13 LXC installer with storage, template, bridge and IPv4 configuration prompts.
- Proxmox installer helper tests and Packaging Check syntax coverage.

### Changed
- Improved mobile and responsive layouts for Home, Events and Presence.
- Refined Events device display options, active filter/grouping indicators and grouped table controls.
- Refreshed Swagger/API documentation, including Events cursor parameters and configuration endpoints.
- Hardened runtime startup so service bind/startup failures exit nonzero and can be retried by systemd.
- Updated the Proxmox installer candidate to install `v0.1.0-beta.2`.

### Fixed
- Events cursor reset reactivity loop that could leave the Events table showing zero loaded rows.
- Events filtering and grouping error handling for database/API failure cases.
- Home inline name-edit focus handling.
- Home responsive width and horizontal overflow issues.
- Events multiselect dropdown clipping and Device checkbox semantics.
- Timezone drift in fresh/current Events relative-time display when server and browser timezones differ.
- Proxmox Debian 13 template discovery and static IPv4 validation.

## [v0.1.0-beta.1] - 2026-08-28
### Added
- First public LANnventory beta release.

## Historical Upstream WatchYourLAN Changelog

## [v2.1.4] - 2025-09-10
### Added
- Swagger API docs (`/swagger/index.html`)
- Add host from API [#72](https://github.com/aceberg/WatchYourLAN/issues/72)
- Trigger rescan from API or by pressing `Save` on `Config/Scan settings` [#74](https://github.com/aceberg/WatchYourLAN/issues/74)
- Delete selected hosts [#195](https://github.com/aceberg/WatchYourLAN/issues/195)
- Wake-on-LAN [#135](https://github.com/aceberg/WatchYourLAN/issues/135), [#196](https://github.com/aceberg/WatchYourLAN/issues/196)

## [v2.1.3] - 2025-07-26
### Fixed
- Memory leak bug [#149](https://github.com/aceberg/WatchYourLAN/issues/149)
- Duplicated devices bug [#187](https://github.com/aceberg/WatchYourLAN/issues/187) [#198](https://github.com/aceberg/WatchYourLAN/issues/198)

### Changed
- **DEPRECATED:** `HIST_IN_DB` config option. Now history is always stored in `DB`
- Upd to `go 1.24.5`
- Moved `DB` handling to `GORM`
- Moved to maintained `Shoutrrr`: [github.com/nicholas-fedor/shoutrrr](https://github.com/nicholas-fedor/shoutrrr) ([#197](https://github.com/aceberg/WatchYourLAN/issues/197))

## [v2.1.2] - 2025-03-30
### Fixed
- Edit names bug
- History page full rerenders replaced with only rerendering updated data
- Select options reset

## [v2.1.1] - 2025-03-26
### Fixed
- Filter bug in Chrome

## [v2.1.0] - 2025-03-25
### Added
- Rewrited GUI in `SolidJS` and `TypeScript`
- Prometheus integration [#181](https://github.com/aceberg/WatchYourLAN/pull/181)
- Optimized Docker build [#180](https://github.com/aceberg/WatchYourLAN/pull/180)

### Fixed
- Vite: file names
- Node Path bug

## [v2.0.4] - 2024-10-21
### Added
- Notification test [#147](https://github.com/aceberg/WatchYourLAN/issues/147) 
- API status [#148](https://github.com/aceberg/WatchYourLAN/issues/148) 

### Fixed
- [#101](https://github.com/aceberg/WatchYourLAN/issues/101) 
- The same problem for Theme, Color mode, Log level
- Sort bug in Chrome [#140](https://github.com/aceberg/WatchYourLAN/issues/140) 

## [v2.0.3] - 2024-09-17
### Fixed
- `ARP_STRS_JOINED` should be empty in config file
- Optimized History Trim

## [v2.0.2] - 2024-09-07
### Added
- Remember Refresh setting in browser [#123](https://github.com/aceberg/WatchYourLAN/issues/123)

### Fixed
- Error when `IFACES` are empty
- Sticky sort bug fix
- Bug [#124](https://github.com/aceberg/WatchYourLAN/issues/124)
- Bug [#128](https://github.com/aceberg/WatchYourLAN/issues/128)


## [v2.0.1] - 2024-09-02
### Added
- `Vlans` and `docker0` support [#47](https://github.com/aceberg/WatchYourLAN/issues/47). Thanks [thehijacker](https://github.com/thehijacker)!
- Remember `sort` field
- `InfluxDB` error handling

### Fixed
- Bug [#103](https://github.com/aceberg/WatchYourLAN/issues/103)
- Bug [#104](https://github.com/aceberg/WatchYourLAN/issues/104). Thanks [Steve Clement](https://github.com/SteveClement)!

## [v2.0.0] - 2024-08-30
### Added
- API
- Arguments for `arp-scan` option
- `InfluxDB` export
- `PostgreSQL` or `SQLite` DB options
- Names from DNS

### Changed
- Better UI with JS
- Switched to `gin` web framework
- Reworked DB schema and config variables

