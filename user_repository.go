package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrUserNotFound 表示 users 表里没有这一行。Handler 把它译成 404。
var ErrUserNotFound = errors.New("用户不存在")

func listUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT id, username, password_hash, age
		 FROM users
		 ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Age,
		)
		if err != nil {
			return nil, fmt.Errorf("读取用户数据失败: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取用户数据失败: %w", err)
	}
	return users, nil
}

func getUserByID(ctx context.Context, db *sql.DB, id int) (User, error) {
	var user User
	err := db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash, age
		 FROM users
		 WHERE id = $1`,
		id,
	).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Age,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

func insertUser(ctx context.Context, db *sql.DB, username string, passwordHash string, age int) error {
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO users (username, password_hash, age)
		 VALUES ($1, $2, $3)`,
		username,
		passwordHash,
		age,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

func updateUser(ctx context.Context, db *sql.DB, id int, username string, passwordHash string, age int) error {
	result, err := db.ExecContext(
		ctx,
		`UPDATE users
		 SET username = $1,
		     password_hash = $2,
		     age = $3
		 WHERE id = $4`,
		username,
		passwordHash,
		age,
		id,
	)
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新影响行数失败: %w", err)
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func deleteUser(ctx context.Context, db *sql.DB, id int) error {
	result, err := db.ExecContext(
		ctx,
		`DELETE FROM users
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除影响行数失败: %w", err)
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}
