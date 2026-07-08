package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// ไฟล์นี้ใช้สำหรับสร้าง bcrypt hash จาก password
// ตัวอย่างการรัน:
// go run ./cmd/hash 123456
func main() {
	// ต้องส่ง password เข้ามาเป็น argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/hash your_password")
		return
	}

	password := os.Args[1]

	// สร้าง bcrypt hash
	// bcrypt.DefaultCost คือ cost มาตรฐาน เหมาะกับการใช้งานทั่วไป
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	// แสดง hash เพื่อนำไปใส่ใน field password_hash
	fmt.Println(string(hash))
}