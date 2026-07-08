package database

import (
	"database/sql"
	"fmt"
	"time"

	"cloud-storage-backend/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

// NewMySQL ใช้สร้าง connection ไปยัง MySQL
func NewMySQL(cfg config.Config) (*sql.DB, error) {
	// DSN คือ connection string สำหรับ MySQL
	// parseTime=true ช่วยให้ Go อ่าน field ประเภท time/timestamp ได้ถูกต้อง
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		cfg.MySQLUser,
		cfg.MySQLPassword,
		cfg.MySQLHost,
		cfg.MySQLPort,
		cfg.MySQLDatabase,
	)

	// sql.Open ยังไม่ได้เชื่อมต่อจริงทันที
	// เป็นการเตรียม database handle
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// จำกัดจำนวน connection เพื่อป้องกัน MySQL Too many connections
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Ping เพื่อเช็กว่าเชื่อมต่อ database ได้จริงไหม
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
