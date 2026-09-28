package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	addr := os.Getenv("KAFKA_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9092"
	}
	log.Printf("消费者读取 user.created，Kafka %s", addr)
	for {
		err := readUserCreated(addr)
		if err != nil {
			log.Printf("读取事件失败，稍后重试: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func readUserCreated(addr string) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{addr},
		Topic:       "user.created",
		GroupID:     "usercreated-logger",
		StartOffset: kafka.LastOffset,
		MinBytes:    1,
		MaxBytes:    1e6,
		MaxWait:     500 * time.Millisecond,
	})
	defer reader.Close()

	for {
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			return err
		}
		log.Printf("收到 UserCreated: %s", message.Value)
	}
}
