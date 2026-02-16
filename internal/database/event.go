package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// StoreWebhookEvent stores a webhook event in the database
func (db *DBWrapper) StoreWebhookEvent(ctx context.Context, event *models.OrderedEvent) error {
	var err error
	maxRetries := 3

	var rawPayloadStr string
	if event.RawPayload != nil {
		rawPayloadStr = string(event.RawPayload)
	}

	status := "pending"
	if event.ProcessedAt != nil {
		status = "processed"
	}

	var processedAt interface{}
	if event.ProcessedAt != nil {
		processedAt = event.ProcessedAt.Format(time.RFC3339)
	}

	for range maxRetries {
		_, err = db.db.ExecContext(ctx,
			`INSERT INTO webhook_events (delivery_id, event_type, sequence_id, 
            github_timestamp, received_at, processed_at, raw_payload, status, ordering_key, status_priority) 
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT (delivery_id) DO UPDATE SET
                event_type = excluded.event_type,
                sequence_id = excluded.sequence_id,
                github_timestamp = excluded.github_timestamp,
                received_at = excluded.received_at,
                processed_at = excluded.processed_at,
                raw_payload = excluded.raw_payload,
                status = excluded.status,
                ordering_key = excluded.ordering_key,
                status_priority = excluded.status_priority`,
			event.Sequence.DeliveryID,
			event.EventType,
			event.Sequence.SequenceID,
			event.Sequence.Timestamp.Format(time.RFC3339),
			event.Sequence.ReceivedAt.Format(time.RFC3339),
			processedAt,
			rawPayloadStr,
			status,
			event.OrderingKey,
			event.StatusPriority,
		)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	return err
}

func (db *DBWrapper) GetPendingEventsGrouped(ctx context.Context, limit int) ([]*models.OrderedEvent, error) {
	query := `
        SELECT delivery_id, event_type, sequence_id, github_timestamp, received_at, 
               processed_at, raw_payload, ordering_key, status_priority
        FROM webhook_events 
        WHERE status = 'pending' 
        ORDER BY github_timestamp ASC, ordering_key ASC, status_priority ASC
        LIMIT ?`

	rows, err := db.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []*models.OrderedEvent
	for rows.Next() {
		var event models.OrderedEvent
		var rawPayload string
		var timestampStr, receivedAtStr string
		var processedAt sql.NullString

		err := rows.Scan(
			&event.Sequence.DeliveryID,
			&event.EventType,
			&event.Sequence.SequenceID,
			&timestampStr,
			&receivedAtStr,
			&processedAt,
			&rawPayload,
			&event.OrderingKey,
			&event.StatusPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		event.Sequence.EventID = event.Sequence.DeliveryID
		event.Sequence.Timestamp = parseTime(timestampStr)
		event.Sequence.ReceivedAt = parseTime(receivedAtStr)
		if processedAt.Valid {
			t := parseTime(processedAt.String)
			event.ProcessedAt = &t
		}
		event.RawPayload = []byte(rawPayload)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (db *DBWrapper) GetPendingEventsByAge(ctx context.Context, maxAge time.Duration, limit int) ([]*models.OrderedEvent, error) {
	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)

	query := `
        SELECT delivery_id, event_type, sequence_id, github_timestamp, received_at, 
               processed_at, raw_payload, ordering_key, status_priority
        FROM webhook_events 
        WHERE status = 'pending' AND received_at <= ?
        ORDER BY github_timestamp ASC, ordering_key ASC, status_priority ASC
        LIMIT ?`

	rows, err := db.db.QueryContext(ctx, query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events by age: %w", err)
	}
	defer rows.Close()

	var events []*models.OrderedEvent
	for rows.Next() {
		var event models.OrderedEvent
		var rawPayload string
		var processedAt sql.NullString
		var timestampStr, receivedAtStr string

		err := rows.Scan(
			&event.Sequence.DeliveryID,
			&event.EventType,
			&event.Sequence.SequenceID,
			&timestampStr,
			&receivedAtStr,
			&processedAt,
			&rawPayload,
			&event.OrderingKey,
			&event.StatusPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		event.Sequence.EventID = event.Sequence.DeliveryID
		event.Sequence.Timestamp = parseTime(timestampStr)
		event.Sequence.ReceivedAt = parseTime(receivedAtStr)
		if processedAt.Valid {
			t := parseTime(processedAt.String)
			event.ProcessedAt = &t
		}
		event.RawPayload = []byte(rawPayload)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (db *DBWrapper) MarkEventProcessed(ctx context.Context, deliveryID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.db.ExecContext(ctx,
		"UPDATE webhook_events SET status = 'processed', processed_at = ? WHERE delivery_id = ?",
		now, deliveryID)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}
	return nil
}

func (db *DBWrapper) MarkEventFailed(ctx context.Context, deliveryID string) error {
	_, err := db.db.ExecContext(ctx,
		"UPDATE webhook_events SET status = 'failed' WHERE delivery_id = ?",
		deliveryID)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}
	return nil
}
