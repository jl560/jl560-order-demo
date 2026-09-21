package main

import (
	"context"
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成密码哈希失败: %w", err)
	}
	return string(hash), nil
}

func CreateUser(ctx context.Context, db *sql.DB, req CreateUserRequest) error {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO users (username, password_hash, age)
		 VALUES ($1, $2, $3)`,
		req.Username,
		passwordHash,
		req.Age,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败：%w", err)
	}
	return nil
}

func UpdateUser(ctx context.Context, db *sql.DB, id int, req UpdateUserRequest) (bool, error) {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return false, err
	}

	result, err := db.ExecContext(
		ctx,
		`UPDATE users
		 SET username = $1,
		     password_hash = $2,
		     age = $3
		 WHERE id = $4`,
		req.Username,
		passwordHash,
		req.Age,
		id,
	)
	if err != nil {
		return false, fmt.Errorf("更新用户失败：%w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("获取更新影响行数失败：%w", err)
	}

	return rowsAffected > 0, nil
}

func DeleteUser(ctx context.Context, db *sql.DB, id int) (bool, error) {
	result, err := db.ExecContext(
		ctx,
		`DELETE FROM users
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		return false, fmt.Errorf("删除用户失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("获取删除影响行数失败: %w", err)
	}

	return rowsAffected > 0, nil
}
