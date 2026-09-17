package gdb

import "github.com/godlev/LANnventory/internal/models"

// SelectAllIdentityCorrelationDecisions returns the complete user decision graph
// for read-only logical identity projection.
func SelectAllIdentityCorrelationDecisions() ([]models.IdentityCorrelationDecision, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.IdentityCorrelationDecision
	err = activeDB.Table(identityCorrelationDecisionsTable).
		Order("\"MAC_A\" ASC, \"MAC_B\" ASC").
		Find(&rows).Error
	return rows, err
}
