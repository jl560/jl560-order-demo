package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成密码哈希失败: %w", err)
	}
	return string(hash), nil
}

func ListUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	return listUsers(ctx, db)
}

func GetUser(ctx context.Context, db *sql.DB, id int) (User, error) {
	if user, ok := lookupCachedUser(ctx, id); ok {
		return user, nil
	}
	user, err := getUserByID(ctx, db, id)
	if err != nil {
		return User{}, err
	}
	storeCachedUser(ctx, user)
	return user, nil
}

func CreateUser(ctx context.Context, db *sql.DB, req CreateUserRequest) error {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始创建用户事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	id, err := insertUser(ctx, tx, req.Username, passwordHash, req.Age)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(userCreatedPayload{
		ID:       id,
		Username: req.Username,
		Age:      req.Age,
	})
	if err != nil {
		return fmt.Errorf("生成 UserCreated 失败: %w", err)
	}
	if err := insertOutbox(ctx, tx, "UserCreated", payload); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交创建用户事务失败: %w", err)
	}
	committed = true
	log.Printf("outbox recorded: user %d", id)
	return nil
}

func UpdateUser(ctx context.Context, db *sql.DB, id int, req UpdateUserRequest) error {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return err
	}
	if err := updateUser(ctx, db, id, req.Username, passwordHash, req.Age); err != nil {
		return err
	}
	invalidateCachedUser(ctx, id)
	return nil
}

func DeleteUser(ctx context.Context, db *sql.DB, id int) error {
	if err := deleteUser(ctx, db, id); err != nil {
		return err
	}
	invalidateCachedUser(ctx, id)
	return nil
}
