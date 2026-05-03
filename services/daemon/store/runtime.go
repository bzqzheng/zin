package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RuntimeRepository struct {
	db *sql.DB
}

func NewRuntimeRepository(db *sql.DB) *RuntimeRepository {
	return &RuntimeRepository{db: db}
}

func (r *RuntimeRepository) List() ([]*Runtime, error) {
	rows, err := r.db.Query(`
SELECT id, kind, display_name, binary_path, version_raw, health_status, health_reason, last_checked_at, created_at, updated_at
FROM runtimes
ORDER BY kind ASC`)
	if err != nil {
		return nil, fmt.Errorf("list runtimes: %w", err)
	}
	defer rows.Close()

	var runtimes []*Runtime
	for rows.Next() {
		runtime, err := scanRuntime(rows)
		if err != nil {
			return nil, err
		}
		runtimes = append(runtimes, runtime)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runtimes: %w", err)
	}
	return runtimes, nil
}

func (r *RuntimeRepository) GetByID(id string) (*Runtime, error) {
	row := r.db.QueryRow(`
SELECT id, kind, display_name, binary_path, version_raw, health_status, health_reason, last_checked_at, created_at, updated_at
FROM runtimes
WHERE id = ?`, id)
	runtime, err := scanRuntime(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *RuntimeRepository) GetByKind(kind string) (*Runtime, error) {
	row := r.db.QueryRow(`
SELECT id, kind, display_name, binary_path, version_raw, health_status, health_reason, last_checked_at, created_at, updated_at
FROM runtimes
WHERE kind = ?`, kind)
	runtime, err := scanRuntime(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *RuntimeRepository) Upsert(runtime *Runtime) (*Runtime, error) {
	now := time.Now().UTC()
	existing, err := r.GetByKind(runtime.Kind)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		uid, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate uuid: %w", err)
		}
		runtime.ID = uid.String()
		runtime.CreatedAt = now
	} else {
		runtime.ID = existing.ID
		runtime.CreatedAt = existing.CreatedAt
	}
	runtime.UpdatedAt = now
	if runtime.LastCheckedAt.IsZero() {
		runtime.LastCheckedAt = now
	}

	_, err = r.db.Exec(`
INSERT INTO runtimes (id, kind, display_name, binary_path, version_raw, health_status, health_reason, last_checked_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(kind) DO UPDATE SET
    display_name = excluded.display_name,
    binary_path = excluded.binary_path,
    version_raw = excluded.version_raw,
    health_status = excluded.health_status,
    health_reason = excluded.health_reason,
    last_checked_at = excluded.last_checked_at,
    updated_at = excluded.updated_at`,
		runtime.ID,
		runtime.Kind,
		runtime.DisplayName,
		runtime.BinaryPath,
		runtime.VersionRaw,
		runtime.HealthStatus,
		runtime.HealthReason,
		runtime.LastCheckedAt.Format(time.RFC3339),
		runtime.CreatedAt.Format(time.RFC3339),
		runtime.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert runtime: %w", err)
	}

	return r.GetByKind(runtime.Kind)
}

func (r *RuntimeRepository) Update(runtime *Runtime) error {
	runtime.UpdatedAt = time.Now().UTC()
	if runtime.LastCheckedAt.IsZero() {
		runtime.LastCheckedAt = runtime.UpdatedAt
	}
	result, err := r.db.Exec(`
UPDATE runtimes
SET display_name = ?, binary_path = ?, version_raw = ?, health_status = ?, health_reason = ?, last_checked_at = ?, updated_at = ?
WHERE id = ?`,
		runtime.DisplayName,
		runtime.BinaryPath,
		runtime.VersionRaw,
		runtime.HealthStatus,
		runtime.HealthReason,
		runtime.LastCheckedAt.Format(time.RFC3339),
		runtime.UpdatedAt.Format(time.RFC3339),
		runtime.ID,
	)
	if err != nil {
		return fmt.Errorf("update runtime: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update runtime rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type runtimeScanner interface {
	Scan(dest ...interface{}) error
}

func scanRuntime(scanner runtimeScanner) (*Runtime, error) {
	runtime := &Runtime{}
	var lastCheckedAt, createdAt, updatedAt string
	if err := scanner.Scan(
		&runtime.ID,
		&runtime.Kind,
		&runtime.DisplayName,
		&runtime.BinaryPath,
		&runtime.VersionRaw,
		&runtime.HealthStatus,
		&runtime.HealthReason,
		&lastCheckedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	var err error
	runtime.LastCheckedAt, err = parseTime(lastCheckedAt)
	if err != nil {
		return nil, fmt.Errorf("scan runtime last_checked_at: %w", err)
	}
	runtime.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan runtime created_at: %w", err)
	}
	runtime.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan runtime updated_at: %w", err)
	}
	return runtime, nil
}
