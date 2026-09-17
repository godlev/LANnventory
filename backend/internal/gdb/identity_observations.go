package gdb

import "github.com/godlev/LANnventory/internal/models"

// SelectAllHostAddresses returns the retained MAC/address observation graph.
// It is used by read-only identity analysis and does not change presence state.
func SelectAllHostAddresses() ([]models.HostAddress, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.HostAddress
	err = activeDB.Table(hostAddressesTable).
		Order("\"MAC\" ASC, \"ACTIVE\" DESC, \"LAST_SEEN\" DESC, \"ADDRESS\" ASC").
		Find(&rows).Error
	return rows, err
}

// SelectAllHostDiscoveryEvidence returns current and historical discovered
// identity evidence for read-only correlation analysis.
func SelectAllHostDiscoveryEvidence() ([]models.HostDiscoveryEvidence, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.HostDiscoveryEvidence
	err = activeDB.Table(hostDiscoveryEvidenceTable).
		Order("\"MAC\" ASC, \"ACTIVE\" DESC, \"SOURCE\" ASC, \"KIND\" ASC, \"VALUE\" ASC").
		Find(&rows).Error
	return rows, err
}
