package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TagRepository struct {
	db *sql.DB
	q  sqlRunner
}

func NewTagRepository(db *sql.DB) *TagRepository {
	return newTagRepository(db, db)
}

func newTagRepository(db *sql.DB, q sqlRunner) *TagRepository {
	return &TagRepository{db: db, q: q}
}

func (r *TagRepository) WithTx(fn func(*TagRepository) error) error {
	if r.db == nil {
		return fn(r)
	}
	return WithTx(r.db, func(repos Repositories) error {
		return fn(repos.Tags)
	})
}

func (r *TagRepository) Create(projectID, name, color string) (*Tag, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	name = strings.TrimSpace(name)
	color = strings.TrimSpace(color)
	if color == "" {
		color = "#71717a"
	}

	now := time.Now().UTC()
	tag := &Tag{
		ID:        uid.String(),
		ProjectID: projectID,
		Name:      name,
		Color:     color,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = r.q.Exec(
		`INSERT INTO tags (id, project_id, name, color, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		tag.ID, tag.ProjectID, tag.Name, tag.Color,
		tag.CreatedAt.Format(time.RFC3339), tag.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert tag: %w", err)
	}
	return tag, nil
}

func (r *TagRepository) GetByID(id string) (*Tag, error) {
	tag := &Tag{}
	var createdAt, updatedAt string
	err := r.q.QueryRow(
		"SELECT id, project_id, name, color, created_at, updated_at FROM tags WHERE id = ?",
		id,
	).Scan(&tag.ID, &tag.ProjectID, &tag.Name, &tag.Color, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tag: %w", err)
	}
	if err := parseTagTimes(tag, createdAt, updatedAt); err != nil {
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return tag, nil
}

func (r *TagRepository) ListByProject(projectID string) ([]*Tag, error) {
	rows, err := r.q.Query(
		"SELECT id, project_id, name, color, created_at, updated_at FROM tags WHERE project_id = ? ORDER BY lower(name) ASC",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r *TagRepository) ListByIssue(issueID string) ([]*Tag, error) {
	rows, err := r.q.Query(
		`SELECT t.id, t.project_id, t.name, t.color, t.created_at, t.updated_at
		 FROM tags t
		 JOIN issue_tags it ON it.tag_id = t.id
		 WHERE it.issue_id = ?
		 ORDER BY lower(t.name) ASC`,
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r *TagRepository) Update(tag *Tag) error {
	tag.Name = strings.TrimSpace(tag.Name)
	tag.Color = strings.TrimSpace(tag.Color)
	if tag.Color == "" {
		tag.Color = "#71717a"
	}
	tag.UpdatedAt = time.Now().UTC()
	_, err := r.q.Exec(
		"UPDATE tags SET name = ?, color = ?, updated_at = ? WHERE id = ?",
		tag.Name, tag.Color, tag.UpdatedAt.Format(time.RFC3339), tag.ID,
	)
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	return nil
}

func (r *TagRepository) Delete(id string) error {
	_, err := r.q.Exec("DELETE FROM tags WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

func (r *TagRepository) AttachToIssue(issueID, tagID string) error {
	_, err := r.q.Exec(
		"INSERT OR IGNORE INTO issue_tags (issue_id, tag_id, created_at) VALUES (?, ?, ?)",
		issueID, tagID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("attach issue tag: %w", err)
	}
	return nil
}

func (r *TagRepository) DetachFromIssue(issueID, tagID string) error {
	_, err := r.q.Exec("DELETE FROM issue_tags WHERE issue_id = ? AND tag_id = ?", issueID, tagID)
	if err != nil {
		return fmt.Errorf("detach issue tag: %w", err)
	}
	return nil
}

func scanTags(rows *sql.Rows) ([]*Tag, error) {
	var tags []*Tag
	for rows.Next() {
		tag := &Tag{}
		var createdAt, updatedAt string
		if err := rows.Scan(&tag.ID, &tag.ProjectID, &tag.Name, &tag.Color, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		if err := parseTagTimes(tag, createdAt, updatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, nil
}

func parseTagTimes(tag *Tag, createdAt, updatedAt string) error {
	var err error
	tag.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return err
	}
	tag.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return err
	}
	return nil
}
