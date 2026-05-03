package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type IssueActivityRepository struct {
	db *sql.DB
	q  sqlRunner
}

type CreateIssueActivityInput struct {
	IssueID      string
	ActorID      string
	Type         string
	Summary      string
	MetadataJSON string
}

func NewIssueActivityRepository(db *sql.DB) *IssueActivityRepository {
	return newIssueActivityRepository(db, db)
}

func newIssueActivityRepository(db *sql.DB, q sqlRunner) *IssueActivityRepository {
	return &IssueActivityRepository{db: db, q: q}
}

func (r *IssueActivityRepository) WithTx(fn func(*IssueActivityRepository) error) error {
	if r.db == nil {
		return fn(r)
	}
	return WithTx(r.db, func(repos Repositories) error {
		return fn(repos.Activity)
	})
}

func (r *IssueActivityRepository) Create(input CreateIssueActivityInput) (*IssueActivity, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	input.MetadataJSON = strings.TrimSpace(input.MetadataJSON)
	if input.MetadataJSON == "" {
		input.MetadataJSON = "{}"
	}

	now := time.Now().UTC()
	activity := &IssueActivity{
		ID:           uid.String(),
		IssueID:      input.IssueID,
		ActorID:      input.ActorID,
		Type:         input.Type,
		Summary:      input.Summary,
		MetadataJSON: input.MetadataJSON,
		CreatedAt:    now,
	}

	_, err = r.q.Exec(
		`INSERT INTO issue_activity (id, issue_id, actor_id, type, summary, metadata_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		activity.ID, activity.IssueID, activity.ActorID, activity.Type,
		activity.Summary, activity.MetadataJSON, activity.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert issue activity: %w", err)
	}
	return activity, nil
}

func (r *IssueActivityRepository) GetByID(id string) (*IssueActivity, error) {
	activity := &IssueActivity{}
	var createdAt string
	err := r.q.QueryRow(
		"SELECT id, issue_id, actor_id, type, summary, metadata_json, created_at FROM issue_activity WHERE id = ?",
		id,
	).Scan(&activity.ID, &activity.IssueID, &activity.ActorID, &activity.Type, &activity.Summary, &activity.MetadataJSON, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get issue activity: %w", err)
	}
	activity.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("get issue activity: %w", err)
	}
	return activity, nil
}

func (r *IssueActivityRepository) ListByIssue(issueID string) ([]*IssueActivity, error) {
	rows, err := r.q.Query(
		"SELECT id, issue_id, actor_id, type, summary, metadata_json, created_at FROM issue_activity WHERE issue_id = ? ORDER BY created_at ASC, id ASC",
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue activity: %w", err)
	}
	defer rows.Close()

	var activities []*IssueActivity
	for rows.Next() {
		activity := &IssueActivity{}
		var createdAt string
		if err := rows.Scan(&activity.ID, &activity.IssueID, &activity.ActorID, &activity.Type, &activity.Summary, &activity.MetadataJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan issue activity: %w", err)
		}
		activity.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan issue activity: %w", err)
		}
		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue activity: %w", err)
	}
	return activities, nil
}

func (r *IssueActivityRepository) Update(activity *IssueActivity) error {
	activity.MetadataJSON = strings.TrimSpace(activity.MetadataJSON)
	if activity.MetadataJSON == "" {
		activity.MetadataJSON = "{}"
	}
	_, err := r.q.Exec(
		"UPDATE issue_activity SET actor_id = ?, type = ?, summary = ?, metadata_json = ? WHERE id = ?",
		activity.ActorID, activity.Type, activity.Summary, activity.MetadataJSON, activity.ID,
	)
	if err != nil {
		return fmt.Errorf("update issue activity: %w", err)
	}
	return nil
}

func (r *IssueActivityRepository) Delete(id string) error {
	_, err := r.q.Exec("DELETE FROM issue_activity WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete issue activity: %w", err)
	}
	return nil
}
