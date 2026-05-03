package store

import (
	"database/sql"
	"fmt"
)

type sqlRunner interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type Repositories struct {
	Issues      *IssueRepository
	Tags        *TagRepository
	Comments    *IssueCommentRepository
	Activity    *IssueActivityRepository
	Assignments *IssueAssignmentRepository
}

func WithTx(db *sql.DB, fn func(Repositories) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin store tx: %w", err)
	}
	defer tx.Rollback()

	repos := Repositories{
		Issues:      newIssueRepository(nil, tx),
		Tags:        newTagRepository(nil, tx),
		Comments:    newIssueCommentRepository(nil, tx),
		Activity:    newIssueActivityRepository(nil, tx),
		Assignments: newIssueAssignmentRepository(nil, tx),
	}

	if err := fn(repos); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit store tx: %w", err)
	}
	return nil
}
