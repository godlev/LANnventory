package gdb

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

const relationshipAssertionsTable = "relationship_assertions"

// ErrRelationshipAlreadyExists indicates a unique identity conflict. The
// existing assertion and its note remain unchanged.
var ErrRelationshipAlreadyExists = errors.New("relationship already exists")

// CreateManualRelationship inserts a validated manual claim, not a generic
// writer for Proxmox-derived relationships or third-party source evidence.
// The transaction validates both current entities before inserting.
func CreateManualRelationship(input relationships.ManualCreateInput) (models.RelationshipAssertion, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return models.RelationshipAssertion{}, err
	}
	defer release()

	var saved models.RelationshipAssertion
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		assertion, err := relationships.NewManualAssertion(input, func(endpoint relationships.Endpoint) (bool, error) {
			return relationshipEndpointExistsTx(txDB, endpoint)
		})
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		saved = models.RelationshipAssertion{
			RelationshipType: string(assertion.Type),
			SourceEntityType: string(assertion.Source.EntityType),
			SourceEntityID: assertion.Source.EntityID,
			TargetEntityType: string(assertion.Target.EntityType),
			TargetEntityID: assertion.Target.EntityID,
			Provenance: string(relationships.ProvenanceManual),
			Note: assertion.Note,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// The unique index handles racing requests. A duplicate create must
		// fail rather than silently update the earlier manual note.
		return txDB.Table(relationshipAssertionsTable).Create(&saved).Error
	})
	if err != nil {
		if isRelationshipDuplicate(err) {
			return models.RelationshipAssertion{}, ErrRelationshipAlreadyExists
		}
		return models.RelationshipAssertion{}, err
	}
	return saved, nil
}

// ListManualRelationshipsByHost includes incoming and outgoing claims in ID
// order. Inverse labels are computed by the domain registry, not duplicate rows.
func ListManualRelationshipsByHost(hostID uint64) ([]models.RelationshipAssertion, error) {
	if hostID == 0 {
		return nil, relationships.ErrInvalidEndpoint
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.RelationshipAssertion
	err = activeDB.Table(relationshipAssertionsTable).
		Where(`"PROVENANCE" = ? AND (("SOURCE_ENTITY_TYPE" = ? AND "SOURCE_ENTITY_ID" = ?) OR ("TARGET_ENTITY_TYPE" = ? AND "TARGET_ENTITY_ID" = ?))`,
			string(relationships.ProvenanceManual), string(relationships.EntityHost), hostID, string(relationships.EntityHost), hostID).
		Order(`"ID" ASC`).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.RelationshipAssertion{}
	}
	return rows, nil
}

// DeleteManualRelationship never deletes another source's assertions.
func DeleteManualRelationship(id uint64) error {
	if id == 0 {
		return relationships.ErrInvalidEndpoint
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()
	result := activeDB.Table(relationshipAssertionsTable).
		Where(`"ID" = ? AND "PROVENANCE" = ?`, id, string(relationships.ProvenanceManual)).
		Delete(&models.RelationshipAssertion{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateManualRelationshipNote validates the requested note and current
// endpoints transactionally, without allowing endpoint/provenance changes.
func UpdateManualRelationshipNote(id uint64, note string) (models.RelationshipAssertion, error) {
	if id == 0 {
		return models.RelationshipAssertion{}, relationships.ErrInvalidEndpoint
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return models.RelationshipAssertion{}, err
	}
	defer release()
	var row models.RelationshipAssertion
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		if err := txDB.Table(relationshipAssertionsTable).
			Where(`"ID" = ? AND "PROVENANCE" = ?`, id, string(relationships.ProvenanceManual)).
			First(&row).Error; err != nil {
			return err
		}
		validated, err := relationships.NewManualAssertion(relationships.ManualCreateInput{
			Type: relationships.Type(row.RelationshipType),
			Source: relationships.Endpoint{EntityType: relationships.EntityType(row.SourceEntityType), EntityID: row.SourceEntityID},
			Target: relationships.Endpoint{EntityType: relationships.EntityType(row.TargetEntityType), EntityID: row.TargetEntityID},
			Note: note,
		}, func(endpoint relationships.Endpoint) (bool, error) {
			return relationshipEndpointExistsTx(txDB, endpoint)
		})
		if err != nil {
			return err
		}
		row.Note = validated.Note
		row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		result := txDB.Table(relationshipAssertionsTable).
			Where(`"ID" = ? AND "PROVENANCE" = ?`, id, string(relationships.ProvenanceManual)).
			Updates(map[string]any{"NOTE": row.Note, "UPDATED_AT": row.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return models.RelationshipAssertion{}, err
	}
	return row, nil
}

func relationshipEndpointExistsTx(txDB *gorm.DB, endpoint relationships.Endpoint) (bool, error) {
	if endpoint.EntityID == 0 || endpoint.EntityID > uint64(^uint(0)>>1) {
		return false, relationships.ErrInvalidEndpoint
	}
	var count int64
	switch endpoint.EntityType {
	case relationships.EntityHost:
		err := txDB.Table("now").Where(`"ID" = ?`, endpoint.EntityID).Count(&count).Error
		return count > 0, err
	case relationships.EntityWorkload:
		err := txDB.Table(infrastructureWorkloadsTable).Where(`"ID" = ?`, endpoint.EntityID).Count(&count).Error
		return count > 0, err
	default:
		return false, fmt.Errorf("%w: %s", relationships.ErrInvalidEntityType, endpoint.EntityType)
	}
}

// deleteRelationshipAssertionsForEndpointTx is intentionally transaction-only.
// Invoked on destructive Host/workload deletion, never on offline/retirement.
func deleteRelationshipAssertionsForEndpointTx(txDB *gorm.DB, entityType string, entityID uint64) error {
	if entityID == 0 {
		return errors.New("invalid relationship cleanup endpoint")
	}
	return txDB.Table(relationshipAssertionsTable).
		Where(`("SOURCE_ENTITY_TYPE" = ? AND "SOURCE_ENTITY_ID" = ?) OR ("TARGET_ENTITY_TYPE" = ? AND "TARGET_ENTITY_ID" = ?)`,
			entityType, entityID, entityType, entityID).
		Delete(&models.RelationshipAssertion{}).Error
}

type relationshipSQLState interface { SQLState() string }

// SQLite reports the indexed columns; PostgreSQL exposes SQLSTATE 23505.
// Restrict SQLite text matching to our assertion table to avoid swallowing
// unrelated persistence errors as relationship conflicts.
func isRelationshipDuplicate(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var state relationshipSQLState
	if errors.As(err, &state) && state.SQLState() == "23505" {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed: relationship_assertions.") ||
		strings.Contains(message, "idx_relationship_assertion_identity")
}
