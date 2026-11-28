// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package audit appends rows to audit_logs. Entries are written in the same transaction
// as the change they describe. Metadata never contains passwords, tokens, keys, cookies or
// e-mail addresses.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/db"
)

// Actor types of audit_logs.actor_type.
const (
	ActorUser      = "user"
	ActorSystem    = "system"
	ActorAnonymous = "anonymous"
)

// Entry is one audit row.
type Entry struct {
	ActorType   string
	ActorUserID *uuid.UUID
	DormitoryID *uuid.UUID
	Action      string
	TargetType  string
	TargetID    *uuid.UUID
	RequestID   string
	Network     *netip.Prefix
	Metadata    map[string]any
}

// Write inserts the entry.
func Write(ctx context.Context, q db.Querier, e Entry) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	meta := e.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("audit: metadata: %w", err)
	}
	var target, requestID *string
	if e.TargetType != "" {
		target = &e.TargetType
	}
	if e.RequestID != "" {
		requestID = &e.RequestID
	}
	var network *string
	if e.Network != nil {
		s := e.Network.String()
		network = &s
	}
	_, err = q.Exec(ctx, `
		INSERT INTO audit_logs (id, actor_type, actor_user_id, dormitory_id, action, target_type, target_id, request_id, ip_network, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::cidr, $10)`,
		id, e.ActorType, e.ActorUserID, e.DormitoryID, e.Action, target, e.TargetID, requestID, network, metaJSON)
	if err != nil {
		return fmt.Errorf("audit: insert %s: %w", e.Action, err)
	}
	return nil
}
