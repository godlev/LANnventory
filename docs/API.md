## API
```http
GET /api/all
```
Returns all hosts in `json`.


```http
GET /api/history
```
Returns all History. Not recommended, the output can be a lot.

```http
GET /api/history/:mac/:date
```
Returns only history of a device with this `mac` filtered by `date`. `date` format can be anything from `2` to `2025-07` to `2025-07-26`.

```http
GET /api/history/:mac?num=20
```
Returns only last 20 lines of history of a device with this `mac`.


```http
GET /api/host/:id
```
Returns host with this `id` in `json`.

```http
GET /api/activity
```
Returns retained device events in `json`, ordered newest first by `Date` and then `ID`.

Supported query parameters:

- `limit`: optional page size from `1` to `100`; defaults to `20`.
- `offset`: optional legacy offset pagination value, `0` or greater.
- `beforeDate` and `beforeId`: optional cursor pagination pair. To request the next page, take `Date` and `ID` from the final event returned by the previous page and pass them as `beforeDate=YYYY-MM-DD HH:mm:ss&beforeId=:id`.
- `category`: optional `all`, `connectivity`, or `changes`.
- `eventType`: optional repeatable filter. Supported values are `discovered`, `online`, `offline`, `known`, `unknown`, and `device-type-changed`.
- `mac`: optional repeatable MAC address filter.

Each event keeps `Date` as the persisted server-local timestamp used for ordering and cursor pagination. Newer API responses also include `DateUTC`, a non-persistent UTC RFC3339 timestamp derived from `Date` using the server timezone for timezone-safe UI display.

Cursor pagination requires both `beforeDate` and `beforeId`. A nonzero `offset` cannot be combined with a cursor; `offset=0` with a cursor is accepted. The response is always an array of events, not a cursor wrapper.

```http
GET /api/activity/stats
```
Returns retained activity event counts. Supports repeatable `mac` query filters.

```http
GET /api/activity/devices
```
Returns device options represented in current hosts and retained activity events.

```http
GET /api/export/backup
```
Downloads a portable JSON backup document with current hosts, host history and Events.

The response uses `Content-Disposition: attachment` with a filename like `lannventory-backup-YYYYMMDDTHHMMSSZ.json`.

Backup metadata includes:

- `format`: always `lannventory-backup`
- `formatVersion`: currently `1`
- `createdAt`: UTC RFC3339 timestamp
- `appVersion`: running LANnventory version

The exported data is a logical backup, not a raw database dump. It excludes runtime configuration, notification URLs, database connection strings, InfluxDB tokens and other secrets. Events preserve the stored `Date` value exactly and do not include derived `DateUTC` display data.

```http
GET /api/export/inventory.csv
```
Downloads the current device inventory as CSV. The response uses `Content-Disposition: attachment` with a filename like `lannventory-inventory-YYYYMMDDTHHMMSSZ.csv`.

CSV columns are:

`ID, Name, DNS, Iface, IP, Mac, Hw, Date, Known, Now, DeviceType`

This export includes current inventory only. It does not include host history, Events, configuration or secrets.

```http
GET /api/host/:id/activity
```
Returns recent activity events for one host. Supports optional `limit` from `1` to `100`.


```http
GET /api/port/:addr/:port
```
Gets state of one `port` of `addr`. Returns `true` if port is open or `false` otherwise.
<details>
  <summary>Request example</summary>

```bash
curl http://0.0.0.0:8840/api/port/192.168.2.2/8844
```
</details><br>


```http
GET /api/edit/:id/:name/*known
```
Edit host with ID `id`. Can change `name`. `known` is optional, when set to `toggle` will change Known state.


```http
GET /api/host/del/:id
```
Remove host with ID `id`.


```http
GET /api/notify_test
```
Send test notification.


```http
GET /api/status/*iface
```
Show status (Total number of hosts, online/offline, known/unknown). The `iface` parameter is optional and shows status for one interface only. For all interfaces just call `/api/status/`.
