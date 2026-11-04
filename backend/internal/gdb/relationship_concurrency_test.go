package gdb

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

type concurrentRelationshipCreateResult struct {
	row models.RelationshipAssertion
	err error
}

func TestConcurrentIdenticalRelationshipCreates(t *testing.T) {
	for _, writers := range []int{2, 16, 32} {
		t.Run(fmt.Sprintf("writers_%d", writers), func(t *testing.T) {
			startSelectTestDB(t)
			seedRelationshipHost(t, 1, "AA:BB:CC:DD:39:01")
			seedRelationshipHost(t, 2, "AA:BB:CC:DD:39:02")

			inputs := make([]relationships.ManualCreateInput, writers)
			for i := range inputs {
				inputs[i] = manualRelationship(relationships.TypeManages, 1, 2, "concurrent")
			}
			assertOneCreateAndDuplicateConflicts(t, runConcurrentRelationshipCreates(inputs))
		})
	}
}

func TestConcurrentIdenticalRelationshipCreatesRepeated(t *testing.T) {
	startSelectTestDB(t)
	seedRelationshipHost(t, 1, "AA:BB:CC:DD:39:11")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:39:12")

	for run := 0; run < 100; run++ {
		inputs := make([]relationships.ManualCreateInput, 16)
		for i := range inputs {
			inputs[i] = manualRelationship(relationships.TypeDependsOn, 1, 2, fmt.Sprintf("run-%d", run))
		}
		id := assertOneCreateAndDuplicateConflicts(t, runConcurrentRelationshipCreates(inputs))
		if err := DeleteManualRelationship(id); err != nil {
			t.Fatalf("run %d cleanup: %v", run, err)
		}
	}
}

func TestConcurrentRelationshipCreatesForDifferentHostPairs(t *testing.T) {
	startSelectTestDB(t)
	const writers = 16
	for id := 1; id <= writers*2; id++ {
		seedRelationshipHost(t, id, fmt.Sprintf("AA:BB:CC:39:%02X:%02X", id/256, id%256))
	}
	inputs := make([]relationships.ManualCreateInput, writers)
	for i := range inputs {
		inputs[i] = manualRelationship(relationships.TypeConnectedTo, uint64(i+1), uint64(i+1+writers), fmt.Sprintf("pair-%d", i))
	}
	for _, result := range runConcurrentRelationshipCreates(inputs) {
		if result.err != nil {
			t.Fatalf("different-pair create: %v", result.err)
		}
	}
	var count int64
	if err := db.Table(relationshipAssertionsTable).Count(&count).Error; err != nil || count != writers {
		t.Fatalf("stored relationships=%d err=%v, want %d", count, err, writers)
	}
}

func TestConcurrentRelationshipNoteUpdatesAndDeletes(t *testing.T) {
	startSelectTestDB(t)
	seedRelationshipHost(t, 1, "AA:BB:CC:DD:39:21")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:39:22")
	created, err := CreateManualRelationship(manualRelationship(relationships.TypeManages, 1, 2, "initial"))
	if err != nil {
		t.Fatal(err)
	}

	const writers = 32
	updateErrors := runConcurrentRelationshipMutations(writers, func(i int) error {
		_, err := UpdateManualRelationshipNote(created.ID, fmt.Sprintf("note-%d", i))
		return err
	})
	for _, err := range updateErrors {
		if err != nil {
			t.Fatalf("concurrent note update: %v", err)
		}
	}
	row, err := SelectManualRelationshipByID(created.ID)
	if err != nil || row.Note == "initial" {
		t.Fatalf("updated row=%+v err=%v", row, err)
	}

	deleteErrors := runConcurrentRelationshipMutations(writers, func(int) error {
		return DeleteManualRelationship(created.ID)
	})
	deleted, notFound := 0, 0
	for _, err := range deleteErrors {
		switch {
		case err == nil:
			deleted++
		case errors.Is(err, gorm.ErrRecordNotFound):
			notFound++
		default:
			t.Fatalf("concurrent delete: %v", err)
		}
	}
	if deleted != 1 || notFound != writers-1 {
		t.Fatalf("delete successes=%d notFound=%d", deleted, notFound)
	}
}

func TestConcurrentRelationshipMixedUpdatesAndDeletes(t *testing.T) {
	startSelectTestDB(t)
	seedRelationshipHost(t, 1, "AA:BB:CC:DD:39:31")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:39:32")
	created, err := CreateManualRelationship(manualRelationship(relationships.TypeManages, 1, 2, "initial"))
	if err != nil {
		t.Fatal(err)
	}

	const mutations = 16
	errs := runConcurrentRelationshipMutations(mutations, func(i int) error {
		if i%2 == 0 {
			_, err := UpdateManualRelationshipNote(created.ID, fmt.Sprintf("mixed-%d", i))
			return err
		}
		return DeleteManualRelationship(created.ID)
	})
	deleteSuccesses := 0
	for i, err := range errs {
		if err == nil {
			if i%2 == 1 {
				deleteSuccesses++
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("mixed mutation %d: %v", i, err)
		}
	}
	if deleteSuccesses != 1 {
		t.Fatalf("successful deletes=%d, want 1", deleteSuccesses)
	}
	if _, err := SelectManualRelationshipByID(created.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("relationship survived mixed delete: %v", err)
	}
}

func TestRelationshipDuplicateClassificationRejectsSQLiteBusy(t *testing.T) {
	if isRelationshipDuplicate(errors.New("database is locked (5) (SQLITE_BUSY)")) {
		t.Fatal("SQLite lock error was misclassified as a duplicate")
	}
}

func runConcurrentRelationshipCreates(inputs []relationships.ManualCreateInput) []concurrentRelationshipCreateResult {
	start := make(chan struct{})
	results := make([]concurrentRelationshipCreateResult, len(inputs))
	var wg sync.WaitGroup
	for i := range inputs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index].row, results[index].err = CreateManualRelationship(inputs[index])
		}(i)
	}
	close(start)
	wg.Wait()
	return results
}

func runConcurrentRelationshipMutations(count int, mutate func(int) error) []error {
	start := make(chan struct{})
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			errs[index] = mutate(index)
		}(i)
	}
	close(start)
	wg.Wait()
	return errs
}

func assertOneCreateAndDuplicateConflicts(t *testing.T, results []concurrentRelationshipCreateResult) uint64 {
	t.Helper()
	successes, duplicates := 0, 0
	var createdID uint64
	for _, result := range results {
		switch {
		case result.err == nil:
			successes++
			createdID = result.row.ID
		case errors.Is(result.err, ErrRelationshipAlreadyExists):
			duplicates++
		default:
			t.Fatalf("unexpected concurrent create error: %v", result.err)
		}
	}
	if successes != 1 || duplicates != len(results)-1 {
		t.Fatalf("successes=%d duplicates=%d, want 1 and %d", successes, duplicates, len(results)-1)
	}
	return createdID
}
