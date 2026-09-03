# Change Log
All notable changes to this project will be documented in this file.

## LANnventory Releases

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

