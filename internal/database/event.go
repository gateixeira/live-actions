package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// StoreWebhookEvent stores a webhook event in the database
func (db *DBWrapper) StoreWebhookEvent(event *models.OrderedEvent) error {
	var err error
	maxRetries := 3

	// Convert raw payload to JSON
	var rawPayloadJSON interface{}
	if event.RawPayload != nil {
		rawPayloadJSON = json.RawMessage(event.RawPayload)
	}

	// Always start as pending
	status := "pending"
	if event.ProcessedAt != nil {
		status = "processed"
	}

	for range maxRetries {
		_, err = DB.Exec(
			`INSERT INTO webhook_events (delivery_id, event_type, sequence_id, 
            github_timestamp, received_at, processed_at, raw_payload, status, ordering_key, status_priority) 
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
            ON CONFLICT (delivery_id) DO UPDATE SET
                event_type = EXCLUDED.event_type,
                sequence_id = EXCLUDED.sequence_id,
                github_timestamp = EXCLUDED.github_timestamp,
                received_at = EXCLUDED.received_at,
                processed_at = EXCLUDED.processed_at,
                raw_payload = EXCLUDED.raw_payload,
                status = EXCLUDED.status,
                ordering_key = EXCLUDED.ordering_key,
                status_priority = EXCLUDED.status_priority`,
			event.Sequence.DeliveryID,
			event.EventType,
			event.Sequence.SequenceID,
			event.Sequence.Timestamp,
			event.Sequence.ReceivedAt,
			event.ProcessedAt,
			rawPayloadJSON,
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

func (db *DBWrapper) GetPendingEventsGrouped(limit int) ([]*models.OrderedEvent, error) {
	query := `
        SELECT delivery_id, event_type, sequence_id, github_timestamp, received_at, 
               processed_at, raw_payload, ordering_key, status_priority
        FROM webhook_events 
        WHERE status = 'pending' 
        ORDER BY github_timestamp ASC, ordering_key ASC, status_priority ASC
        LIMIT $1`

	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []*models.OrderedEvent
	for rows.Next() {
		var event models.OrderedEvent
		var rawPayloadJSON json.RawMessage
		var processedAt *time.Time

		err := rows.Scan(
			&event.Sequence.DeliveryID,
			&event.EventType,
			&event.Sequence.SequenceID,
			&event.Sequence.Timestamp,
			&event.Sequence.ReceivedAt,
			&processedAt,
			&rawPayloadJSON,
			&event.OrderingKey,
			&event.StatusPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		event.Sequence.EventID = event.Sequence.DeliveryID
		event.ProcessedAt = processedAt
		event.RawPayload = []byte(rawPayloadJSON)

		events = append(events, &event)
	}

	return events, nil
}

func (db *DBWrapper) GetPendingEventsByAge(maxAge time.Duration, limit int) ([]*models.OrderedEvent, error) {
	cutoff := time.Now().Add(-maxAge)

	query := `
        SELECT delivery_id, event_type, sequence_id, github_timestamp, received_at, 
               processed_at, raw_payload, ordering_key, status_priority
        FROM webhook_events 
        WHERE status = 'pending' AND received_at <= $1
        ORDER BY github_timestamp ASC, ordering_key ASC, status_priority ASC
        LIMIT $2`

	rows, err := DB.Query(query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events by age: %w", err)
	}
	defer rows.Close()

	var events []*models.OrderedEvent
	for rows.Next() {
		var event models.OrderedEvent
		var rawPayloadJSON json.RawMessage
		var processedAt *time.Time

		err := rows.Scan(
			&event.Sequence.DeliveryID,
			&event.EventType,
			&event.Sequence.SequenceID,
			&event.Sequence.Timestamp,
			&event.Sequence.ReceivedAt,
			&processedAt,
			&rawPayloadJSON,
			&event.OrderingKey,
			&event.StatusPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		event.Sequence.EventID = event.Sequence.DeliveryID
		event.ProcessedAt = processedAt
		event.RawPayload = []byte(rawPayloadJSON)

		events = append(events, &event)
	}

	return events, nil
}

func (db *DBWrapper) MarkEventProcessed(deliveryID string) error {
	now := time.Now()
	_, err := DB.Exec(
		"UPDATE webhook_events SET status = 'processed', processed_at = $1 WHERE delivery_id = $2",
		now, deliveryID)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}
	return nil
}

func (db *DBWrapper) MarkEventFailed(deliveryID string) error {
	_, err := DB.Exec(
		"UPDATE webhook_events SET status = 'failed' WHERE delivery_id = $1",
		deliveryID)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}
	return nil
}
