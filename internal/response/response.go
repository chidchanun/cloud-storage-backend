package response

import (
	"encoding/json"
	"net/http"
)

// JSON ใช้ส่ง response แบบ JSON กลับไปหา client
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(data)
}

// Error ใช้ส่ง error response ให้ format เหมือนกันทั้งระบบ
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, map[string]string{
		"error": message,
	})
}