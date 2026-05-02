package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(name, description string) (*Project, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	now := time.Now().UTC()
	p := &Project{
		ID:          uid.String(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = r.db.Exec(
		"INSERT INTO projects (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		p.ID, p.Name, p.Description, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	return p, nil
}

func (r *ProjectRepository) GetByID(id string) (*Project, error) {
	p := &Project{}
	var createdAt, updatedAt string
	err := r.db.QueryRow(
		"SELECT id, name, description, created_at, updated_at FROM projects WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return p, nil
}

func (r *ProjectRepository) List() ([]*Project, error) {
	rows, err := r.db.Query("SELECT id, name, description, created_at, updated_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p := &Project{}
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		projects = append(projects, p)
	}
	return projects, nil
}

func (r *ProjectRepository) Update(p *Project) error {
	p.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(
		"UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?",
		p.Name, p.Description, p.UpdatedAt.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
