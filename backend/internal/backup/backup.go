package backup

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

const (
	Format        = "lannventory-backup"
	FormatVersion = 2
)

// Document is the stable, versioned logical backup format.
type Document struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"formatVersion"`
	CreatedAt     string `json:"createdAt"`
	AppVersion    string `json:"appVersion"`
	Data          Data   `json:"data"`
}

// Data contains persisted application data only. It intentionally excludes
// configuration and secrets so exports are portable across database backends.
type Data struct {
	CurrentHosts []Host         `json:"currentHosts"`
	History      []Host         `json:"history"`
	Events       []Event        `json:"events"`
	HostMetadata []HostMetadata `json:"hostMetadata"`
}

// Host mirrors the currently persisted host columns in the now/history tables.
type Host struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	DNS        string `json:"dns"`
	Iface      string `json:"iface"`
	IP         string `json:"ip"`
	Mac        string `json:"mac"`
	Hw         string `json:"hw"`
	Date       string `json:"date"`
	Known      int    `json:"known"`
	Now        int    `json:"now"`
	DeviceType string `json:"deviceType"`
}

// Event mirrors the currently persisted event columns. DateUTC is intentionally
// omitted because it is derived display data, not persisted source data.
type Event struct {
	ID         int    `json:"id"`
	HostID     int    `json:"hostId"`
	Mac        string `json:"mac"`
	Name       string `json:"name"`
	EventType  string `json:"eventType"`
	Date       string `json:"date"`
	IP         string `json:"ip"`
	Iface      string `json:"iface"`
	DeviceType string `json:"deviceType"`
	OldValue   string `json:"oldValue"`
	NewValue   string `json:"newValue"`
}

// HostMetadata is the portable metadata backup representation.
type HostMetadata struct {
	Mac      string   `json:"mac"`
	Owner    string   `json:"owner"`
	Location string   `json:"location"`
	Notes    string   `json:"notes"`
	Tags     []string `json:"tags"`
	Pinned   bool     `json:"pinned"`
}

// InventoryHost is the current-inventory CSV representation.
type InventoryHost struct {
	Host
	Owner    string
	Location string
	Notes    string
	Tags     []string
	Pinned   bool
}

var InventoryCSVHeader = []string{
	"ID",
	"Name",
	"DNS",
	"Iface",
	"IP",
	"Mac",
	"Hw",
	"Date",
	"Known",
	"Now",
	"DeviceType",
	"Owner",
	"Location",
	"Notes",
	"Tags",
	"Pinned",
}

func NewDocument(data Data, appVersion string, createdAt time.Time) Document {
	return Document{
		Format:        Format,
		FormatVersion: FormatVersion,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		AppVersion:    appVersion,
		Data:          normalizeData(data),
	}
}

func DataFromModels(currentHosts, history []models.Host, events []models.HostEvent, hostMetadata []models.HostMetadata) Data {
	data := Data{
		CurrentHosts: make([]Host, 0, len(currentHosts)),
		History:      make([]Host, 0, len(history)),
		Events:       make([]Event, 0, len(events)),
		HostMetadata: make([]HostMetadata, 0, len(hostMetadata)),
	}

	for _, host := range currentHosts {
		data.CurrentHosts = append(data.CurrentHosts, HostFromModel(host))
	}
	for _, host := range history {
		data.History = append(data.History, HostFromModel(host))
	}
	for _, event := range events {
		data.Events = append(data.Events, EventFromModel(event))
	}
	for _, metadata := range hostMetadata {
		data.HostMetadata = append(data.HostMetadata, HostMetadataFromModel(metadata))
	}

	return data
}

func HostFromModel(host models.Host) Host {
	return Host{
		ID:         host.ID,
		Name:       host.Name,
		DNS:        host.DNS,
		Iface:      host.Iface,
		IP:         host.IP,
		Mac:        host.Mac,
		Hw:         host.Hw,
		Date:       host.Date,
		Known:      host.Known,
		Now:        host.Now,
		DeviceType: host.DeviceType,
	}
}

func EventFromModel(event models.HostEvent) Event {
	return Event{
		ID:         event.ID,
		HostID:     event.HostID,
		Mac:        event.Mac,
		Name:       event.Name,
		EventType:  event.EventType,
		Date:       event.Date,
		IP:         event.IP,
		Iface:      event.Iface,
		DeviceType: event.DeviceType,
		OldValue:   event.OldValue,
		NewValue:   event.NewValue,
	}
}

func HostMetadataFromModel(metadata models.HostMetadata) HostMetadata {
	return HostMetadata{
		Mac:      metadata.Mac,
		Owner:    metadata.Owner,
		Location: metadata.Location,
		Notes:    metadata.Notes,
		Tags:     models.DecodeMetadataTags(metadata.TagsJSON),
		Pinned:   metadata.Pinned,
	}
}

func InventoryHostFromModel(host models.Host) InventoryHost {
	return InventoryHost{
		Host:     HostFromModel(host),
		Owner:    host.Owner,
		Location: host.Location,
		Notes:    host.Notes,
		Tags:     host.Tags,
		Pinned:   host.Pinned,
	}
}

func WriteInventoryCSV(writer io.Writer, hosts []InventoryHost) error {
	csvWriter := csv.NewWriter(writer)

	if err := csvWriter.Write(InventoryCSVHeader); err != nil {
		return err
	}
	for _, host := range hosts {
		if err := csvWriter.Write([]string{
			strconv.Itoa(host.ID),
			host.Name,
			host.DNS,
			host.Iface,
			host.IP,
			host.Mac,
			host.Hw,
			host.Date,
			strconv.Itoa(host.Known),
			strconv.Itoa(host.Now),
			host.DeviceType,
			host.Owner,
			host.Location,
			host.Notes,
			strings.Join(host.Tags, "; "),
			strconv.FormatBool(host.Pinned),
		}); err != nil {
			return err
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func normalizeData(data Data) Data {
	if data.CurrentHosts == nil {
		data.CurrentHosts = []Host{}
	}
	if data.History == nil {
		data.History = []Host{}
	}
	if data.Events == nil {
		data.Events = []Event{}
	}
	if data.HostMetadata == nil {
		data.HostMetadata = []HostMetadata{}
	}

	return data
}
