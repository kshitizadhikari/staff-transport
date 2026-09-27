// Package audit records important state changes in the audit_logs table.
//
// Entries are written inside the same transaction as the mutation they
// describe, so an audit record cannot exist without its change and vice versa.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Entry describes one audited action. ActorUserID and EntityID may be nil for
// system actions; Metadata must not contain secrets or precise location data.
type Entry struct {
	ActorUserID *string
	Action      string
	EntityType  string
	EntityID    *string
	Metadata    map[string]any
}

// Record writes an audit entry using db, which may be a transaction.
func Record(ctx context.Context, db *gorm.DB, e Entry) error {
	metadata := "{}"
	if len(e.Metadata) > 0 {
		raw, err := json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
		metadata = string(raw)
	}

	return db.WithContext(ctx).Table("audit_logs").Create(map[string]any{
		"id":            uuid.NewString(),
		"actor_user_id": e.ActorUserID,
		"action":        e.Action,
		"entity_type":   e.EntityType,
		"entity_id":     e.EntityID,
		"metadata":      gorm.Expr("?::jsonb", metadata),
	}).Error
}

// StrPtr is a helper for optional string fields in entries.
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
