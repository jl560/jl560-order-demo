package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const userCreatedTopic = "user.created"

func StartOutboxPublisher(ctx context.Context, db *sql.DB, addr string) {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(addr),
		Topic:                  userCreatedTopic,
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
		BatchTimeout:           10 * time.Millisecond,
	}
	go func() {
		defer writer.Close()
		publishPendingOutbox(db, writer)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishPendingOutbox(db, writer)
			}
		}
	}()
}

func publishPendingOutbox(db *sql.DB, writer *kafka.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pending, err := listUnpublishedOutbox(ctx, db, 10)
	if err != nil {
		log.Printf("读取 outbox 失败: %v", err)
		return
	}
	for _, row := range pending {
		err := writer.WriteMessages(ctx, kafka.Message{Value: []byte(row.Payload)})
		if err != nil {
			log.Printf("发送 UserCreated 失败，保留在 outbox: %v", err)
			return
		}
		if err := markOutboxPublished(ctx, db, row.ID); err != nil {
			log.Printf("UserCreated 已发送，但标记 outbox 失败: %v", err)
			return
		}
		log.Printf("outbox published: id=%d", row.ID)
	}
}
