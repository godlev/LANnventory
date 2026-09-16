package gdb

import (
	"context"
	"time"
)

// DatabaseHealth describes the live database connection currently serving LANnventory.
type DatabaseHealth struct {
	Connected bool
	Backend   string
	Error     string
}

// GetDatabaseHealth checks the active DB handle without opening a second connection.
func GetDatabaseHealth() DatabaseHealth {
	active, release, err := acquireDB()
	if err != nil {
		return DatabaseHealth{Error: redactDatabaseError(err)}
	}
	defer release()

	health := DatabaseHealth{Backend: active.Dialector.Name()}

	sqlDB, err := active.DB()
	if err != nil {
		health.Error = redactDatabaseError(err)
		return health
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		health.Error = redactDatabaseError(err)
		return health
	}

	health.Connected = true
	return health
}
