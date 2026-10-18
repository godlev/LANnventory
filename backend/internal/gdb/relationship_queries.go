package gdb

import (
	"strings"

	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

// SelectRelationshipHostTargets searches ONLY current inventory Hosts. This
// deliberately does not consult historical IP/MAC observations, identity
// candidates, or workload inventory. Each request is SQL-bounded.
func SelectRelationshipHostTargets(query string, excludeHostID uint64, limit int) ([]models.Host, error) {
	if limit < 1 || limit > 20 {
		limit = 20
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.Host{}, nil
	}
	// LIKE wildcards in user input represent literal characters.
	escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.ToLower(query))
	pattern := "%" + escaped + "%"
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	selectDB := activeDB.Table("now").
		Select(`"ID", "NAME", "DNS", "IP", "MAC", "DEVICE_TYPE"`).
		Where(`LOWER("NAME") LIKE ? ESCAPE '\' OR LOWER("DNS") LIKE ? ESCAPE '\' OR LOWER("IP") LIKE ? ESCAPE '\' OR LOWER("MAC") LIKE ? ESCAPE '\'`,
			pattern, pattern, pattern, pattern)
	if excludeHostID > 0 {
		selectDB = selectDB.Where(`"ID" <> ?`, excludeHostID)
	}
	var rows []models.Host
	err = selectDB.Order(`"NAME" ASC`).Order(`"ID" ASC`).Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.Host{}
	}
	return rows, nil
}

// SelectManualRelationshipByID is read-only and never returns other-source
// assertions through a manual relationship API.
func SelectManualRelationshipByID(id uint64) (models.RelationshipAssertion, error) {
	var row models.RelationshipAssertion
	if id == 0 {
		return row, gorm.ErrRecordNotFound
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return row, err
	}
	defer release()
	err = activeDB.Table(relationshipAssertionsTable).
		Where(`"ID" = ? AND "PROVENANCE" = ?`, id, "manual").
		First(&row).Error
	return row, err
}
