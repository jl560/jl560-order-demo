package main

import (
	"context"
	"database/sql"
)

// userCreatedPayload 是 UserCreated 事件的内容。只有客户端本来就能看见的字段。
type userCreatedPayload struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Age      int    `json:"age"`
}

type unpublishedOutbox struct {
	ID      int64
	Payload string
}

func ensureOutboxTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS outbox (
			id           BIGSERIAL PRIMARY KEY,
			event_type   TEXT NOT NULL,
			payload      TEXT NOT NULL,
			published_at TIMESTAMPTZ
		)`)
	return err
}

func insertOutbox(ctx context.Context, tx *sql.Tx, eventType string, payload []byte) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (event_type, payload)
		 VALUES ($1, $2)`,
		eventType,
		string(payload),
	)
	if err != nil {
		return err
	}
	return nil
}

func listUnpublishedOutbox(ctx context.Context, db *sql.DB, limit int) ([]unpublishedOutbox, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT id, payload
		 FROM outbox
		 WHERE published_at IS NULL
		 ORDER BY id
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []unpublishedOutbox
	for rows.Next() {
		var row unpublishedOutbox
		if err := rows.Scan(&row.ID, &row.Payload); err != nil {
			return nil, err
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pending, nil
}

func markOutboxPublished(ctx context.Context, db *sql.DB, id int64) error {
	_, err := db.ExecContext(
		ctx,
		`UPDATE outbox
		 SET published_at = now()
		 WHERE id = $1`,
		id,
	)
	return err
}
