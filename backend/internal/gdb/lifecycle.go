package gdb

import (
	"errors"

	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostLifecycleTable = "host_lifecycle"

var errEmptyLifecycleMAC = errors.New("lifecycle mac is empty")

// RecordHostObservation records a successful scanner observation for a MAC.
func RecordHostObservation(mac, observedAt string) error {
	if mac == "" {
		return errEmptyLifecycleMAC
	}
	if observedAt == "" {
		return nil
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		lifecycle, ok, err := selectHostLifecycleByMAC(txDB, mac)
		if err != nil {
			return err
		}
		if !ok {
			lifecycle = models.HostLifecycle{
				Mac:                mac,
				FirstSeen:          observedAt,
				LastSeen:           observedAt,
				FirstSeenEstimated: false,
			}
			return txDB.Table(hostLifecycleTable).Create(&lifecycle).Error
		}

		if lifecycle.FirstSeen == "" {
			lifecycle.FirstSeen = observedAt
			lifecycle.FirstSeenEstimated = false
		}
		lifecycle.LastSeen = observedAt
		return txDB.Table(hostLifecycleTable).Save(&lifecycle).Error
	})
}

// EnsureHostLifecyclePlaceholder creates an empty lifecycle row when a current
// host is entered manually before any scanner observation exists.
func EnsureHostLifecyclePlaceholder(mac string) error {
	if mac == "" {
		return nil
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		_, ok, err := selectHostLifecycleByMAC(txDB, mac)
		if err != nil || ok {
			return err
		}

		return txDB.Table(hostLifecycleTable).Create(&models.HostLifecycle{Mac: mac}).Error
	})
}

// SelectHostLifecycleByMAC returns lifecycle state for one MAC address.
func SelectHostLifecycleByMAC(mac string) (lifecycle models.HostLifecycle, ok bool, err error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return lifecycle, false, err
	}
	defer release()

	return selectHostLifecycleByMAC(activeDB, mac)
}

// SelectHostLifecycleForMACs returns all lifecycle records matching the provided MACs.
func SelectHostLifecycleForMACs(macs []string) (map[string]models.HostLifecycle, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	return selectHostLifecycleForMACs(activeDB, macs)
}

func deleteHostLifecycleByMAC(activeDB *gorm.DB, mac string) error {
	if mac == "" {
		return nil
	}

	return activeDB.Table(hostLifecycleTable).
		Where("\"MAC\" = ?", mac).
		Delete(&models.HostLifecycle{}).Error
}

func backfillHostLifecycle(activeDB *gorm.DB) error {
	var currentHosts []models.Host
	if err := activeDB.Table("now").
		Select("\"MAC\", \"DATE\"").
		Where("\"MAC\" <> ?", "").
		Order("\"MAC\" ASC").
		Find(&currentHosts).Error; err != nil {
		return err
	}

	macs := make([]string, 0, len(currentHosts))
	currentByMAC := make(map[string]models.Host, len(currentHosts))
	for _, host := range currentHosts {
		if host.Mac == "" {
			continue
		}
		if _, exists := currentByMAC[host.Mac]; exists {
			continue
		}
		currentByMAC[host.Mac] = host
		macs = append(macs, host.Mac)
	}
	if len(macs) == 0 {
		return nil
	}

	existing, err := selectHostLifecycleForMACs(activeDB, macs)
	if err != nil {
		return err
	}
	missingMACs := make([]string, 0, len(macs))
	for _, mac := range macs {
		if _, exists := existing[mac]; !exists {
			missingMACs = append(missingMACs, mac)
		}
	}
	if len(missingMACs) == 0 {
		return nil
	}

	earliestDiscovered, err := selectEarliestScannerDiscoveredEventDatesByMAC(activeDB, missingMACs)
	if err != nil {
		return err
	}
	earliestHistory, err := selectEarliestObservedHistoryDatesByMAC(activeDB, missingMACs)
	if err != nil {
		return err
	}

	lifecycles := make([]models.HostLifecycle, 0, len(missingMACs))
	for _, mac := range missingMACs {
		host := currentByMAC[mac]
		firstSeen := earliestDiscovered[mac]
		if firstSeen == "" {
			firstSeen = earliestHistory[mac]
		}
		if firstSeen == "" {
			firstSeen = host.Date
		}

		lifecycles = append(lifecycles, models.HostLifecycle{
			Mac:                mac,
			FirstSeen:          firstSeen,
			LastSeen:           host.Date,
			FirstSeenEstimated: firstSeen != "",
		})
	}
	if len(lifecycles) == 0 {
		return nil
	}

	return activeDB.Table(hostLifecycleTable).Create(&lifecycles).Error
}

func enrichHostsWithLifecycle(activeDB *gorm.DB, hosts []models.Host) error {
	lifecycleByMAC, err := selectHostLifecycleForHosts(activeDB, hosts)
	if err != nil {
		return err
	}

	for i := range hosts {
		lifecycle, ok := lifecycleByMAC[hosts[i].Mac]
		models.ApplyHostLifecycle(&hosts[i], lifecycle, ok)
	}

	return nil
}

func selectHostLifecycleForHosts(activeDB *gorm.DB, hosts []models.Host) (map[string]models.HostLifecycle, error) {
	macs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host.Mac != "" {
			macs = append(macs, host.Mac)
		}
	}

	return selectHostLifecycleForMACs(activeDB, macs)
}

func selectHostLifecycleForMACs(activeDB *gorm.DB, macs []string) (map[string]models.HostLifecycle, error) {
	lifecycleByMAC := make(map[string]models.HostLifecycle, len(macs))
	macs = normalizeMacs(macs)
	if len(macs) == 0 {
		return lifecycleByMAC, nil
	}

	var rows []models.HostLifecycle
	if err := activeDB.Table(hostLifecycleTable).
		Where("\"MAC\" IN ?", macs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		lifecycleByMAC[row.Mac] = row
	}

	return lifecycleByMAC, nil
}

func selectHostLifecycleByMAC(activeDB *gorm.DB, mac string) (lifecycle models.HostLifecycle, ok bool, err error) {
	if mac == "" {
		return lifecycle, false, nil
	}

	err = activeDB.Table(hostLifecycleTable).
		Where("\"MAC\" = ?", mac).
		First(&lifecycle).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return lifecycle, false, nil
	}
	if err != nil {
		return lifecycle, false, err
	}

	return lifecycle, true, nil
}

func selectEarliestScannerDiscoveredEventDatesByMAC(activeDB *gorm.DB, macs []string) (map[string]string, error) {
	type dateRow struct {
		Mac  string `gorm:"column:MAC"`
		Date string `gorm:"column:DATE"`
	}

	macs = normalizeMacs(macs)
	dates := make(map[string]string, len(macs))
	if len(macs) == 0 {
		return dates, nil
	}

	var rows []dateRow
	if err := activeDB.Table("events").
		Select("\"MAC\", MIN(\"DATE\") as \"DATE\"").
		Where("\"EVENT_TYPE\" = ?", string(models.EventDiscovered)).
		Where("\"MAC\" IN ?", macs).
		Where("\"IFACE\" <> ?", "").
		Group("MAC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		dates[row.Mac] = row.Date
	}
	return dates, nil
}

func selectEarliestObservedHistoryDatesByMAC(activeDB *gorm.DB, macs []string) (map[string]string, error) {
	type dateRow struct {
		Mac  string `gorm:"column:MAC"`
		Date string `gorm:"column:DATE"`
	}

	macs = normalizeMacs(macs)
	dates := make(map[string]string, len(macs))
	if len(macs) == 0 {
		return dates, nil
	}

	var rows []dateRow
	if err := activeDB.Table("history").
		Select("\"MAC\", MIN(\"DATE\") as \"DATE\"").
		Where("\"MAC\" IN ?", macs).
		Where("\"NOW\" = ?", 1).
		Group("MAC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		dates[row.Mac] = row.Date
	}
	return dates, nil
}
