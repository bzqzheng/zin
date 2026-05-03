package store

import (
	"fmt"
	"time"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Agent struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Role             string    `json:"role"`
	Status           string    `json:"status"`
	RuntimeID        string    `json:"runtime_id"`
	RuntimeName      string    `json:"runtime_name"`
	RuntimeStatus    string    `json:"runtime_status"`
	Model            string    `json:"model"`
	Instructions     string    `json:"instructions"`
	Assignable       bool      `json:"assignable"`
	AssignableReason string    `json:"assignable_reason"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Runtime struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	Command       string    `json:"command"`
	Path          string    `json:"path"`
	Version       string    `json:"version"`
	Status        string    `json:"status"`
	StatusMessage string    `json:"status_message"`
	LastCheckedAt string    `json:"last_checked_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Issue struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Identifier  string    `json:"identifier"`
	Position    int       `json:"position"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	AssigneeID  string    `json:"assignee_id"`
	CreatorID   string    `json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Tag struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IssueComment struct {
	ID        string    `json:"id"`
	IssueID   string    `json:"issue_id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IssueActivity struct {
	ID           string    `json:"id"`
	IssueID      string    `json:"issue_id"`
	ActorID      string    `json:"actor_id"`
	Type         string    `json:"type"`
	Summary      string    `json:"summary"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type IssueAssignment struct {
	ID                 string     `json:"id"`
	IssueID            string     `json:"issue_id"`
	AgentID            string     `json:"agent_id"`
	AgentName          string     `json:"agent_name"`
	RuntimeID          string     `json:"runtime_id"`
	RuntimeName        string     `json:"runtime_name"`
	RuntimeStatus      string     `json:"runtime_status"`
	RequestedBy        string     `json:"requested_by"`
	SourceType         string     `json:"source_type"`
	SourceID           string     `json:"source_id"`
	ClientRequestID    string     `json:"client_request_id"`
	RequestFingerprint string     `json:"request_fingerprint"`
	Status             string     `json:"status"`
	DedupeKey          string     `json:"dedupe_key"`
	RequestedAt        time.Time  `json:"requested_at"`
	AcceptedAt         *time.Time `json:"accepted_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	FailedAt           *time.Time `json:"failed_at"`
	CancelledAt        *time.Time `json:"cancelled_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}
