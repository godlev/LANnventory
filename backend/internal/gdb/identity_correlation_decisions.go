package gdb

import (
	"errors"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const identityCorrelationDecisionsTable = "identity_correlation_decisions"

// SetIdentityCorrelationDecision creates or replaces one explicit user decision
// for an unordered MAC pair. Historical host observations are never modified.
func SetIdentityCorrelationDecision(macA, macB, decision, changedAt string) (models.IdentityCorrelationDecision, error) {
	left, right, err := canonicalCorrelationPair(macA, macB)
	if err != nil {
		return models.IdentityCorrelationDecision{}, err
	}

	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != models.IdentityCorrelationConfirmed && decision != models.IdentityCorrelationRejected {
		return models.IdentityCorrelationDecision{}, errors.New("correlation decision must be confirmed or rejected")
	}
	changedAt = strings.TrimSpace(changedAt)
	if changedAt == "" {
		return models.IdentityCorrelationDecision{}, errors.New("correlation decision timestamp is required")
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.IdentityCorrelationDecision{}, err
	}
	defer release()

	var result models.IdentityCorrelationDecision
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var existing models.IdentityCorrelationDecision
		err := txDB.Table(identityCorrelationDecisionsTable).
			Where("\"MAC_A\" = ? AND \"MAC_B\" = ?", left, right).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = models.IdentityCorrelationDecision{
				MacA:      left,
				MacB:      right,
				Decision:  decision,
				CreatedAt: changedAt,
				UpdatedAt: changedAt,
			}
			return txDB.Table(identityCorrelationDecisionsTable).Create(&result).Error
		}
		if err != nil {
			return err
		}

		existing.Decision = decision
		existing.UpdatedAt = changedAt
		result = existing
		return txDB.Table(identityCorrelationDecisionsTable).Save(&existing).Error
	})
	return result, err
}

// DeleteIdentityCorrelationDecision clears a user's explicit decision for one
// MAC pair without touching either identity or its retained history.
func DeleteIdentityCorrelationDecision(macA, macB string) error {
	left, right, err := canonicalCorrelationPair(macA, macB)
	if err != nil {
		return err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Table(identityCorrelationDecisionsTable).
		Where("\"MAC_A\" = ? AND \"MAC_B\" = ?", left, right).
		Delete(&models.IdentityCorrelationDecision{}).Error
}

// SelectIdentityCorrelationDecisionsForMAC returns all explicit user decisions
// involving one MAC identity in deterministic order.
func SelectIdentityCorrelationDecisionsForMAC(mac string) ([]models.IdentityCorrelationDecision, error) {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.IdentityCorrelationDecision
	err = activeDB.Table(identityCorrelationDecisionsTable).
		Where("\"MAC_A\" = ? OR \"MAC_B\" = ?", canonical, canonical).
		Order("\"UPDATED_AT\" DESC, \"MAC_A\" ASC, \"MAC_B\" ASC").
		Find(&rows).Error
	return rows, err
}

// SelectIdentityCorrelationDecision returns one pair decision, if present.
func SelectIdentityCorrelationDecision(macA, macB string) (models.IdentityCorrelationDecision, bool, error) {
	left, right, err := canonicalCorrelationPair(macA, macB)
	if err != nil {
		return models.IdentityCorrelationDecision{}, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.IdentityCorrelationDecision{}, false, err
	}
	defer release()

	var row models.IdentityCorrelationDecision
	err = activeDB.Table(identityCorrelationDecisionsTable).
		Where("\"MAC_A\" = ? AND \"MAC_B\" = ?", left, right).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.IdentityCorrelationDecision{}, false, nil
	}
	return row, err == nil, err
}

func canonicalCorrelationPair(macA, macB string) (string, string, error) {
	left, err := identity.NormalizeMAC(macA)
	if err != nil {
		return "", "", err
	}
	right, err := identity.NormalizeMAC(macB)
	if err != nil {
		return "", "", err
	}
	if left == right {
		return "", "", errors.New("correlation pair requires two different MAC identities")
	}

	pair := []string{left, right}
	sort.Strings(pair)
	return pair[0], pair[1], nil
}
