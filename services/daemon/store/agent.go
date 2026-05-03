package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AgentRepository struct {
	db *sql.DB
	q  sqlRunner
}

func NewAgentRepository(db *sql.DB) *AgentRepository {
	return newAgentRepository(db, db)
}

func newAgentRepository(db *sql.DB, q sqlRunner) *AgentRepository {
	return &AgentRepository{db: db, q: q}
}

type CreateAgentInput struct {
	Name         string
	Role         string
	RuntimeID    string
	ModelHint    string
	Instructions string
	IsAssignable bool
}

func (r *AgentRepository) Create(name, role string) (*Agent, error) {
	return r.CreateWithInput(CreateAgentInput{Name: name, Role: role})
}

func (r *AgentRepository) CreateWithInput(input CreateAgentInput) (*Agent, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	now := time.Now().UTC()
	a := &Agent{
		ID:           uid.String(),
		Name:         input.Name,
		Role:         input.Role,
		Status:       "offline",
		RuntimeID:    input.RuntimeID,
		ModelHint:    input.ModelHint,
		Instructions: input.Instructions,
		IsAssignable: input.IsAssignable,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = r.q.Exec(
		`INSERT INTO agents (id, name, role, status, runtime_id, model_hint, instructions, is_assignable, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Role, a.Status, a.RuntimeID, a.ModelHint, a.Instructions, boolToInt(a.IsAssignable),
		a.CreatedAt.Format(time.RFC3339), a.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	return a, nil
}

func (r *AgentRepository) GetByID(id string) (*Agent, error) {
	a := &Agent{}
	var createdAt, updatedAt string
	var isAssignable int
	err := r.q.QueryRow(
		`SELECT id, name, role, status, runtime_id, model_hint, instructions, is_assignable, created_at, updated_at
		 FROM agents WHERE id = ?`,
		id,
	).Scan(&a.ID, &a.Name, &a.Role, &a.Status, &a.RuntimeID, &a.ModelHint, &a.Instructions, &isAssignable, &createdAt, &updatedAt)
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
	a.IsAssignable = isAssignable != 0
	return a, nil
}

func (r *AgentRepository) List() ([]*Agent, error) {
	return r.ListWithFilters(AgentFilters{})
}

type AgentFilters struct {
	Assignable *bool
}

func (r *AgentRepository) ListWithFilters(filters AgentFilters) ([]*Agent, error) {
	query := `SELECT id, name, role, status, runtime_id, model_hint, instructions, is_assignable, created_at, updated_at FROM agents`
	var args []any
	if filters.Assignable != nil {
		query += " WHERE is_assignable = ?"
		args = append(args, boolToInt(*filters.Assignable))
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a := &Agent{}
		var createdAt, updatedAt string
		var isAssignable int
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.Status, &a.RuntimeID, &a.ModelHint, &a.Instructions, &isAssignable, &createdAt, &updatedAt); err != nil {
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
		a.IsAssignable = isAssignable != 0
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return agents, nil
}

func (r *AgentRepository) Update(a *Agent) error {
	a.UpdatedAt = time.Now().UTC()
	_, err := r.q.Exec(
		`UPDATE agents
		 SET name = ?, role = ?, status = ?, runtime_id = ?, model_hint = ?, instructions = ?, is_assignable = ?, updated_at = ?
		 WHERE id = ?`,
		a.Name, a.Role, a.Status, a.RuntimeID, a.ModelHint, a.Instructions, boolToInt(a.IsAssignable),
		a.UpdatedAt.Format(time.RFC3339), a.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	return nil
}

func (r *AgentRepository) Delete(id string) error {
	_, err := r.q.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
