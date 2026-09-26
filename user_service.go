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

func ListUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	return listUsers(ctx, db)
}

func GetUser(ctx context.Context, db *sql.DB, id int) (User, error) {
	return getUserByID(ctx, db, id)
}

func CreateUser(ctx context.Context, db *sql.DB, req CreateUserRequest) error {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return err
	}
	return insertUser(ctx, db, req.Username, passwordHash, req.Age)
}

func UpdateUser(ctx context.Context, db *sql.DB, id int, req UpdateUserRequest) error {
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return err
	}
	return updateUser(ctx, db, id, req.Username, passwordHash, req.Age)
}

func DeleteUser(ctx context.Context, db *sql.DB, id int) error {
	return deleteUser(ctx, db, id)
}
