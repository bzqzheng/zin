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

func (r *RuntimeRepository) Create(name, kind, command, path, version, status, statusMessage string) (*Runtime, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}
	now := time.Now().UTC()
	if status == "" {
		status = "unknown"
	}
	runtime := &Runtime{
		ID:            uid.String(),
		Name:          name,
		Kind:          kind,
		Command:       command,
		Path:          path,
		Version:       version,
		Status:        status,
		StatusMessage: statusMessage,
		LastCheckedAt: now.Format(time.RFC3339),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = r.db.Exec(
		`INSERT INTO runtimes (id, name, kind, command, path, version, status, status_message, last_checked_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runtime.ID,
		runtime.Name,
		runtime.Kind,
		runtime.Command,
		runtime.Path,
		runtime.Version,
		runtime.Status,
		runtime.StatusMessage,
		runtime.LastCheckedAt,
		runtime.CreatedAt.Format(time.RFC3339),
		runtime.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert runtime: %w", err)
	}

	return runtime, nil
}

func (r *RuntimeRepository) GetByID(id string) (*Runtime, error) {
	runtime := &Runtime{}
	var createdAt, updatedAt string
	err := r.db.QueryRow(
		`SELECT id, name, kind, command, path, version, status, status_message, last_checked_at, created_at, updated_at
		 FROM runtimes WHERE id = ?`,
		id,
	).Scan(
		&runtime.ID,
		&runtime.Name,
		&runtime.Kind,
		&runtime.Command,
		&runtime.Path,
		&runtime.Version,
		&runtime.Status,
		&runtime.StatusMessage,
		&runtime.LastCheckedAt,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get runtime: %w", err)
	}
	runtime.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("get runtime: %w", err)
	}
	runtime.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get runtime: %w", err)
	}
	return runtime, nil
}

func (r *RuntimeRepository) List() ([]*Runtime, error) {
	rows, err := r.db.Query(
		`SELECT id, name, kind, command, path, version, status, status_message, last_checked_at, created_at, updated_at
		 FROM runtimes ORDER BY name ASC, created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list runtimes: %w", err)
	}
	defer rows.Close()

	var runtimes []*Runtime
	for rows.Next() {
		runtime := &Runtime{}
		var createdAt, updatedAt string
		if err := rows.Scan(
			&runtime.ID,
			&runtime.Name,
			&runtime.Kind,
			&runtime.Command,
			&runtime.Path,
			&runtime.Version,
			&runtime.Status,
			&runtime.StatusMessage,
			&runtime.LastCheckedAt,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan runtime: %w", err)
		}
		runtime.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan runtime: %w", err)
		}
		runtime.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan runtime: %w", err)
		}
		runtimes = append(runtimes, runtime)
	}

	return runtimes, nil
}

func (r *RuntimeRepository) Update(runtime *Runtime) error {
	runtime.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(
		`UPDATE runtimes
		 SET name = ?, kind = ?, command = ?, path = ?, version = ?, status = ?, status_message = ?, last_checked_at = ?, updated_at = ?
		 WHERE id = ?`,
		runtime.Name,
		runtime.Kind,
		runtime.Command,
		runtime.Path,
		runtime.Version,
		runtime.Status,
		runtime.StatusMessage,
		runtime.LastCheckedAt,
		runtime.UpdatedAt.Format(time.RFC3339),
		runtime.ID,
	)
	if err != nil {
		return fmt.Errorf("update runtime: %w", err)
	}
	return nil
}
