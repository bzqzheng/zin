package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type IssueRepository struct {
	db *sql.DB
	q  sqlRunner
}

func NewIssueRepository(db *sql.DB) *IssueRepository {
	return newIssueRepository(db, db)
}

func newIssueRepository(db *sql.DB, q sqlRunner) *IssueRepository {
	return &IssueRepository{db: db, q: q}
}

func (r *IssueRepository) WithTx(fn func(*IssueRepository) error) error {
	if r.db == nil {
		return fn(r)
	}
	return WithTx(r.db, func(repos Repositories) error {
		return fn(repos.Issues)
	})
}

type IssueFilters struct {
	Status   string
	Priority string
}

func (r *IssueRepository) Create(projectID, title, description, status, priority string) (*Issue, error) {
	if r.db != nil {
		var issue *Issue
		err := r.WithTx(func(txRepo *IssueRepository) error {
			var err error
			issue, err = txRepo.create(projectID, title, description, status, priority)
			return err
		})
		return issue, err
	}
	return r.create(projectID, title, description, status, priority)
}

func (r *IssueRepository) create(projectID, title, description, status, priority string) (*Issue, error) {
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

	var position int
	if err := r.q.QueryRow(
		"SELECT COALESCE(MAX(position), 0) + 1 FROM issues WHERE project_id = ?",
		projectID,
	).Scan(&position); err != nil {
		return nil, fmt.Errorf("next issue position: %w", err)
	}

	identifier := fmt.Sprintf("ISSUE-%d", position)
	now := time.Now().UTC()
	iss := &Issue{
		ID:          uid.String(),
		ProjectID:   projectID,
		Identifier:  identifier,
		Position:    position,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = r.q.Exec(
		`INSERT INTO issues (id, project_id, identifier, position, title, description, status, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		iss.ID, iss.ProjectID, iss.Identifier, iss.Position, iss.Title, iss.Description,
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
	err := r.q.QueryRow(
		`SELECT id, project_id, identifier, position, title, description, status, priority,
		        assignee_id, creator_id, created_at, updated_at
		 FROM issues WHERE id = ?`, id,
	).Scan(&iss.ID, &iss.ProjectID, &iss.Identifier, &iss.Position, &iss.Title, &iss.Description,
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

func (r *IssueRepository) ListByProject(projectID string, filters IssueFilters) ([]*Issue, error) {
	query := `SELECT id, project_id, identifier, position, title, description, status, priority,
	                 assignee_id, creator_id, created_at, updated_at
	          FROM issues WHERE project_id = ?`
	args := []interface{}{projectID}
	if filters.Status != "" {
		query += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.Priority != "" {
		query += " AND priority = ?"
		args = append(args, filters.Priority)
	}
	query += " ORDER BY position ASC"

	rows, err := r.q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	return scanIssues(rows)
}

func (r *IssueRepository) Update(iss *Issue) error {
	iss.UpdatedAt = time.Now().UTC()
	_, err := r.q.Exec(
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
	_, err := r.q.Exec("UPDATE issues SET status = ?, updated_at = ? WHERE id = ?", status, now, id)
	if err != nil {
		return fmt.Errorf("update issue status: %w", err)
	}
	return nil
}

func (r *IssueRepository) Delete(id string) error {
	_, err := r.q.Exec("DELETE FROM issues WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	return nil
}

type IssueFieldChange struct {
	Field        string
	OldValue     string
	NewValue     string
	ActivityType string
}

func MeaningfulIssueChanges(before, after *Issue) []IssueFieldChange {
	if before == nil || after == nil {
		return nil
	}

	candidates := []IssueFieldChange{
		{Field: "title", OldValue: before.Title, NewValue: after.Title, ActivityType: "issue.updated"},
		{Field: "description", OldValue: before.Description, NewValue: after.Description, ActivityType: "issue.updated"},
		{Field: "status", OldValue: before.Status, NewValue: after.Status, ActivityType: "status.changed"},
		{Field: "priority", OldValue: before.Priority, NewValue: after.Priority, ActivityType: "issue.updated"},
		{Field: "assignee_id", OldValue: before.AssigneeID, NewValue: after.AssigneeID, ActivityType: "assignee.changed"},
		{Field: "creator_id", OldValue: before.CreatorID, NewValue: after.CreatorID, ActivityType: "creator.changed"},
	}

	changes := make([]IssueFieldChange, 0, len(candidates))
	for _, change := range candidates {
		change.OldValue = strings.TrimSpace(change.OldValue)
		change.NewValue = strings.TrimSpace(change.NewValue)
		if change.OldValue != change.NewValue {
			changes = append(changes, change)
		}
	}
	return changes
}

func ActivityInputsForIssueChanges(issueID, actorID string, changes []IssueFieldChange) ([]CreateIssueActivityInput, error) {
	inputs := make([]CreateIssueActivityInput, 0, len(changes))
	for _, change := range changes {
		metadata, err := json.Marshal(map[string]string{
			"field":     change.Field,
			"old_value": change.OldValue,
			"new_value": change.NewValue,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal issue change metadata: %w", err)
		}
		inputs = append(inputs, CreateIssueActivityInput{
			IssueID:      issueID,
			ActorID:      actorID,
			Type:         change.ActivityType,
			Summary:      fmt.Sprintf("%s changed", change.Field),
			MetadataJSON: string(metadata),
		})
	}
	return inputs, nil
}

func scanIssues(rows *sql.Rows) ([]*Issue, error) {
	var issues []*Issue
	for rows.Next() {
		iss := &Issue{}
		var createdAt, updatedAt string
		var err error
		if err = rows.Scan(&iss.ID, &iss.ProjectID, &iss.Identifier, &iss.Position, &iss.Title,
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
