package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestSelectEventsFilteredCursorUsesDateAndID(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:   1,
		Name: "router",
		Mac:  "AA:BB:CC:DD:EE:01",
	}

	seedSelectTestEvent(t, host, "2026-08-31 22:00:00", "first-same-date")
	seedSelectTestEvent(t, host, "2026-08-31 22:00:00", "second-same-date")
	seedSelectTestEvent(t, host, "2026-08-31 22:00:00", "third-same-date")
	seedSelectTestEvent(t, host, "2026-08-31 21:59:59", "older")

	firstPage, ok := SelectEventsFiltered(EventQuery{Limit: 2})
	if !ok {
		t.Fatal("SelectEventsFiltered first page failed")
	}
	assertSelectTestMarkers(t, firstPage, []string{"third-same-date", "second-same-date"})

	secondPage, ok := SelectEventsFiltered(EventQuery{
		Limit:      2,
		BeforeDate: firstPage[len(firstPage)-1].Date,
		BeforeID:   firstPage[len(firstPage)-1].ID,
	})
	if !ok {
		t.Fatal("SelectEventsFiltered cursor page failed")
	}
	assertSelectTestMarkers(t, secondPage, []string{"first-same-date", "older"})
}

func startSelectTestDB(t *testing.T) {
	t.Helper()

	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "select-test.db"),
	})
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})
}

func seedSelectTestEvent(t *testing.T, host models.Host, date string, marker string) {
	t.Helper()

	event := models.NewHostEvent(host, models.EventOnline, marker, "")
	event.Date = date
	if err := AddEvent(event); err != nil {
		t.Fatalf("AddEvent %s: %v", marker, err)
	}
}

func assertSelectTestMarkers(t *testing.T, events []models.HostEvent, want []string) {
	t.Helper()

	if len(events) != len(want) {
		t.Fatalf("events len = %d, want %d: %+v", len(events), len(want), events)
	}
	for i, marker := range want {
		if events[i].OldValue != marker {
			t.Fatalf("events[%d].OldValue = %q, want %q; events: %+v", i, events[i].OldValue, marker, events)
		}
	}
}


func TestSelectByDateRangeUsesHalfOpenBounds(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:55"
	for _, date := range []string{
		"2026-09-25 20:59:59",
		"2026-09-25 21:00:00",
		"2026-09-26 20:59:59",
		"2026-09-26 21:00:00",
	} {
		Update("history", models.Host{
			Name: "presence-test",
			Mac:  mac,
			Date: date,
		})
	}

	rows, ok := SelectByDateRange(mac, "2026-09-25 21:00:00", "2026-09-26 21:00:00")
	if !ok {
		t.Fatal("SelectByDateRange failed")
	}
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2: %+v", len(rows), rows)
	}
	if rows[0].Date != "2026-09-25 21:00:00" || rows[1].Date != "2026-09-26 20:59:59" {
		t.Fatalf("range rows = %+v, want half-open range bounds", rows)
	}
}
