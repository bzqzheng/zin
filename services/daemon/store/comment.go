package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type IssueCommentRepository struct {
	db *sql.DB
	q  sqlRunner
}

func NewIssueCommentRepository(db *sql.DB) *IssueCommentRepository {
	return newIssueCommentRepository(db, db)
}

func newIssueCommentRepository(db *sql.DB, q sqlRunner) *IssueCommentRepository {
	return &IssueCommentRepository{db: db, q: q}
}

func (r *IssueCommentRepository) WithTx(fn func(*IssueCommentRepository) error) error {
	if r.db == nil {
		return fn(r)
	}
	return WithTx(r.db, func(repos Repositories) error {
		return fn(repos.Comments)
	})
}

func (r *IssueCommentRepository) Create(issueID, authorID, body string) (*IssueComment, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	now := time.Now().UTC()
	comment := &IssueComment{
		ID:        uid.String(),
		IssueID:   issueID,
		AuthorID:  authorID,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = r.q.Exec(
		`INSERT INTO issue_comments (id, issue_id, author_id, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		comment.ID, comment.IssueID, comment.AuthorID, comment.Body,
		comment.CreatedAt.Format(time.RFC3339), comment.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert issue comment: %w", err)
	}
	return comment, nil
}

func (r *IssueCommentRepository) GetByID(id string) (*IssueComment, error) {
	comment := &IssueComment{}
	var createdAt, updatedAt string
	err := r.q.QueryRow(
		"SELECT id, issue_id, author_id, body, created_at, updated_at FROM issue_comments WHERE id = ?",
		id,
	).Scan(&comment.ID, &comment.IssueID, &comment.AuthorID, &comment.Body, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get issue comment: %w", err)
	}
	if err := parseIssueCommentTimes(comment, createdAt, updatedAt); err != nil {
		return nil, fmt.Errorf("get issue comment: %w", err)
	}
	return comment, nil
}

func (r *IssueCommentRepository) ListByIssue(issueID string) ([]*IssueComment, error) {
	rows, err := r.q.Query(
		"SELECT id, issue_id, author_id, body, created_at, updated_at FROM issue_comments WHERE issue_id = ? ORDER BY created_at ASC, id ASC",
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue comments: %w", err)
	}
	defer rows.Close()

	var comments []*IssueComment
	for rows.Next() {
		comment := &IssueComment{}
		var createdAt, updatedAt string
		if err := rows.Scan(&comment.ID, &comment.IssueID, &comment.AuthorID, &comment.Body, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan issue comment: %w", err)
		}
		if err := parseIssueCommentTimes(comment, createdAt, updatedAt); err != nil {
			return nil, fmt.Errorf("scan issue comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue comments: %w", err)
	}
	return comments, nil
}

func (r *IssueCommentRepository) Update(comment *IssueComment) error {
	comment.UpdatedAt = time.Now().UTC()
	_, err := r.q.Exec(
		"UPDATE issue_comments SET body = ?, updated_at = ? WHERE id = ?",
		comment.Body, comment.UpdatedAt.Format(time.RFC3339), comment.ID,
	)
	if err != nil {
		return fmt.Errorf("update issue comment: %w", err)
	}
	return nil
}

func (r *IssueCommentRepository) Delete(id string) error {
	_, err := r.q.Exec("DELETE FROM issue_comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete issue comment: %w", err)
	}
	return nil
}

func parseIssueCommentTimes(comment *IssueComment, createdAt, updatedAt string) error {
	var err error
	comment.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return err
	}
	comment.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return err
	}
	return nil
}
