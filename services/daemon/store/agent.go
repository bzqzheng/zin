package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AgentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) Create(name, role string) (*Agent, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	now := time.Now().UTC()
	a := &Agent{
		ID:        uid.String(),
		Name:      name,
		Role:      role,
		Status:    "offline",
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = r.db.Exec(
		"INSERT INTO agents (id, name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		a.ID, a.Name, a.Role, a.Status, a.CreatedAt.Format(time.RFC3339), a.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	return a, nil
}

func (r *AgentRepository) GetByID(id string) (*Agent, error) {
	a := &Agent{}
	var createdAt, updatedAt string
	err := r.db.QueryRow(
		"SELECT id, name, role, status, created_at, updated_at FROM agents WHERE id = ?",
		id,
	).Scan(&a.ID, &a.Name, &a.Role, &a.Status, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	a.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	a.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return a, nil
}

func (r *AgentRepository) List() ([]*Agent, error) {
	rows, err := r.db.Query("SELECT id, name, role, status, created_at, updated_at FROM agents ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a := &Agent{}
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		a.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		a.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (r *AgentRepository) Update(a *Agent) error {
	a.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(
		"UPDATE agents SET name = ?, role = ?, status = ?, updated_at = ? WHERE id = ?",
		a.Name, a.Role, a.Status, a.UpdatedAt.Format(time.RFC3339), a.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	return nil
}

func (r *AgentRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}
