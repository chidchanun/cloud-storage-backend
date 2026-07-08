package models

// GoogleIdentity คือข้อมูลผู้ใช้ที่ผ่านการตรวจสอบจาก Google ID Token แล้ว
type GoogleIdentity struct {
	// Subject คือ Google Account ID จาก claim ชื่อ sub
	// ใช้เป็น identifier หลักของบัญชี Google
	Subject string

	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
	FullName      string
	PictureURL    string
}