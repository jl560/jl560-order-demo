package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	// 把password替换成你自己PG的密码
	connStr := "host=localhost port=5432 user=postgres password=#jL793606 dbname=go_demo sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("打开连接失败:", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("ping数据库失败:", err)
	}
	fmt.Println("✅ Go成功连接PostgreSQL go_demo数据库！ - main.go:24")
}
