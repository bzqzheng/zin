package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type IssueRepository struct {
	db *sql.DB
}

func NewIssueRepository(db *sql.DB) *IssueRepository {
	return &IssueRepository{db: db}
}

func (r *IssueRepository) Create(projectID, identifier, title, description, status, priority string) (*Issue, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	if status == "" {
		status = "todo"
	}
	if priority == "" {
		priority = "medium"
	}

	now := time.Now().UTC()
	iss := &Issue{
		ID:          uid.String(),
		ProjectID:   projectID,
		Identifier:  identifier,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = r.db.Exec(
		`INSERT INTO issues (id, project_id, identifier, title, description, status, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		iss.ID, iss.ProjectID, iss.Identifier, iss.Title, iss.Description,
		iss.Status, iss.Priority, iss.CreatedAt.Format(time.RFC3339), iss.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert issue: %w", err)
	}

	return iss, nil
}

func (r *IssueRepository) GetByID(id string) (*Issue, error) {
	iss := &Issue{}
	var createdAt, updatedAt string
	err := r.db.QueryRow(
		`SELECT id, project_id, identifier, title, description, status, priority,
		        assignee_id, creator_id, created_at, updated_at
		 FROM issues WHERE id = ?`, id,
	).Scan(&iss.ID, &iss.ProjectID, &iss.Identifier, &iss.Title, &iss.Description,
		&iss.Status, &iss.Priority, &iss.AssigneeID, &iss.CreatorID, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	iss.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	iss.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	return iss, nil
}

func (r *IssueRepository) ListByProject(projectID string) ([]*Issue, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, identifier, title, description, status, priority,
		        assignee_id, creator_id, created_at, updated_at
		 FROM issues WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	return scanIssues(rows)
}

func (r *IssueRepository) Update(iss *Issue) error {
	iss.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(
		`UPDATE issues SET title = ?, description = ?, status = ?, priority = ?,
		       assignee_id = ?, creator_id = ?, updated_at = ?
		 WHERE id = ?`,
		iss.Title, iss.Description, iss.Status, iss.Priority,
		iss.AssigneeID, iss.CreatorID, iss.UpdatedAt.Format(time.RFC3339), iss.ID,
	)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

func (r *IssueRepository) UpdateStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec("UPDATE issues SET status = ?, updated_at = ? WHERE id = ?", status, now, id)
	if err != nil {
		return fmt.Errorf("update issue status: %w", err)
	}
	return nil
}

func (r *IssueRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM issues WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	return nil
}

func scanIssues(rows *sql.Rows) ([]*Issue, error) {
	var issues []*Issue
	for rows.Next() {
		iss := &Issue{}
		var createdAt, updatedAt string
		var err error
		if err = rows.Scan(&iss.ID, &iss.ProjectID, &iss.Identifier, &iss.Title,
			&iss.Description, &iss.Status, &iss.Priority,
			&iss.AssigneeID, &iss.CreatorID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		iss.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		iss.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, iss)
	}
	return issues, nil
}
