package gdb

import (
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

	activeDB, release, err := acquireDB()
	if err != nil {
		return backup.Data{}, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
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

		return nil
	})
	if err != nil {
		return backup.Data{}, err
	}

	return backup.DataFromModels(currentHosts, history, events), nil
}
