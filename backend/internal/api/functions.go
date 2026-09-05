package api

import (
	"errors"
	"strconv"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

var errInvalidHostID = errors.New("invalid host id")

func getHostByID(idStr string) (models.Host, error) {

	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		return models.Host{}, errInvalidHostID
	}

	host := gdb.SelectByID(id)
	if host.ID < 1 {
		return models.Host{}, errInvalidHostID
	}

	return host, nil
}

func getHostWithMetadataByID(idStr string) (models.Host, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		return models.Host{}, errInvalidHostID
	}

	host, err := gdb.SelectHostWithMetadataByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Host{}, errInvalidHostID
	}
	if err != nil {
		return models.Host{}, err
	}
	if host.ID < 1 {
		return models.Host{}, errInvalidHostID
	}

	return host, nil
}
