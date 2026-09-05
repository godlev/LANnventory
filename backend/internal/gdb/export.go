package gdb

import (
	"database/sql"

	"github.com/godlev/LANnventory/internal/backup"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExportData returns one logical read snapshot of the persisted application data.
func ExportData() (backup.Data, error) {
	var currentHosts []models.Host
	var history []models.Host
	var events []models.HostEvent
	var hostMetadata []models.HostMetadata

	activeDB, release, err := acquireDB()
	if err != nil {
		return backup.Data{}, err
	}
	defer release()

	err = exportTransaction(activeDB, func(txDB *gorm.DB) error {
		idAscending := clause.OrderByColumn{Column: clause.Column{Name: "ID"}}

		if err := txDB.Table("now").Order(idAscending).Find(&currentHosts).Error; err != nil {
			return err
		}
		if err := txDB.Table("history").Order(idAscending).Find(&history).Error; err != nil {
			return err
		}
		if err := txDB.Table("events").Order(idAscending).Find(&events).Error; err != nil {
			return err
		}
		if err := txDB.Table("host_metadata").Order(clause.OrderByColumn{Column: clause.Column{Name: "MAC"}}).Find(&hostMetadata).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return backup.Data{}, err
	}

	return backup.DataFromModels(currentHosts, history, events, hostMetadata), nil
}

// ExportCurrentHosts returns enriched current inventory without reading history tables.
func ExportCurrentHosts() ([]backup.InventoryHost, error) {
	var currentHosts []models.Host

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	idAscending := clause.OrderByColumn{Column: clause.Column{Name: "ID"}}
	if err := activeDB.Table("now").Order(idAscending).Find(&currentHosts).Error; err != nil {
		return nil, err
	}
	if err := enrichHostsWithMetadata(activeDB, currentHosts); err != nil {
		return nil, err
	}

	hosts := make([]backup.InventoryHost, 0, len(currentHosts))
	for _, host := range currentHosts {
		hosts = append(hosts, backup.InventoryHostFromModel(host))
	}

	return hosts, nil
}

func exportTransaction(db *gorm.DB, fn func(txDB *gorm.DB) error) error {
	options := exportTransactionOptions(db.Dialector.Name())
	if options == nil {
		return db.Transaction(fn)
	}

	return db.Transaction(fn, options)
}

func exportTransactionOptions(dialect string) *sql.TxOptions {
	if dialect != "postgres" {
		return nil
	}

	return &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}
}
