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

func (r *AgentRepository) Create(name, role, runtimeID, model, instructions string) (*Agent, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	now := time.Now().UTC()
	a := &Agent{
		ID:           uid.String(),
		Name:         name,
		Role:         role,
		Status:       "offline",
		RuntimeID:    runtimeID,
		Model:        model,
		Instructions: instructions,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = r.db.Exec(
		`INSERT INTO agents (id, name, role, status, runtime_id, model, instructions, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID,
		a.Name,
		a.Role,
		a.Status,
		a.RuntimeID,
		a.Model,
		a.Instructions,
		a.CreatedAt.Format(time.RFC3339),
		a.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	created, err := r.GetByID(a.ID)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *AgentRepository) GetByID(id string) (*Agent, error) {
	a := &Agent{}
	var createdAt, updatedAt string
	err := r.db.QueryRow(
		`SELECT a.id, a.name, a.role, a.status, a.runtime_id, a.model, a.instructions,
		        COALESCE(rt.name, ''), COALESCE(rt.status, ''),
		        a.created_at, a.updated_at
		 FROM agents a
		 LEFT JOIN runtimes rt ON rt.id = a.runtime_id
		 WHERE a.id = ?`,
		id,
	).Scan(
		&a.ID,
		&a.Name,
		&a.Role,
		&a.Status,
		&a.RuntimeID,
		&a.Model,
		&a.Instructions,
		&a.RuntimeName,
		&a.RuntimeStatus,
		&createdAt,
		&updatedAt,
	)
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
	a.applyAssignableGate()
	return a, nil
}

func (r *AgentRepository) List(assignableOnly bool) ([]*Agent, error) {
	rows, err := r.db.Query(
		`SELECT a.id, a.name, a.role, a.status, a.runtime_id, a.model, a.instructions,
		        COALESCE(rt.name, ''), COALESCE(rt.status, ''),
		        a.created_at, a.updated_at
		 FROM agents a
		 LEFT JOIN runtimes rt ON rt.id = a.runtime_id
		 ORDER BY a.created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a := &Agent{}
		var createdAt, updatedAt string
		if err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.Role,
			&a.Status,
			&a.RuntimeID,
			&a.Model,
			&a.Instructions,
			&a.RuntimeName,
			&a.RuntimeStatus,
			&createdAt,
			&updatedAt,
		); err != nil {
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
		a.applyAssignableGate()
		if assignableOnly && !a.Assignable {
			continue
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (r *AgentRepository) Update(a *Agent) error {
	a.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(
		`UPDATE agents
		 SET name = ?, role = ?, status = ?, runtime_id = ?, model = ?, instructions = ?, updated_at = ?
		 WHERE id = ?`,
		a.Name,
		a.Role,
		a.Status,
		a.RuntimeID,
		a.Model,
		a.Instructions,
		a.UpdatedAt.Format(time.RFC3339),
		a.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	a.applyAssignableGate()
	return nil
}

func (r *AgentRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

func (a *Agent) applyAssignableGate() {
	switch {
	case a.RuntimeID == "":
		a.Assignable = false
		a.AssignableReason = "Bind this agent to a runtime before assigning work."
	case a.RuntimeStatus != "healthy":
		a.Assignable = false
		if a.RuntimeStatus == "" {
			a.AssignableReason = "The bound runtime is unavailable. Rebind the agent to a healthy runtime."
		} else {
			a.AssignableReason = "The bound runtime is " + a.RuntimeStatus + ". Revalidate or choose a healthy runtime before assigning work."
		}
	default:
		a.Assignable = true
		a.AssignableReason = ""
	}
}
