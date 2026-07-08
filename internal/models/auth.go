package models

// LoginRequest คือข้อมูลที่ client ส่งมาเพื่อ login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse คือข้อมูลที่ server ส่งกลับหลัง login สำเร็จ
type LoginResponse struct {
	AccessToken string       `json:"accessToken"`
	User        UserResponse `json:"user"`
}

// RegisterRequest คือข้อมูลที่ client ส่งมาเพื่อสมัครสมาชิก
type RegisterRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Phone     string `json:"phone"`
}

// SetPasswordRequest ใช้สำหรับบัญชี Google ที่ยังไม่มีรหัสผ่าน
type SetPasswordRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// UserResponse คือข้อมูล user ที่ปลอดภัยสำหรับส่งกลับไปหา client
// ไม่มี password_hash
type UserResponse struct {
	ID            int     `json:"id"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Email         string  `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	HasPassword   bool    `json:"has_password"`
	PicturePath   *string `json:"picture_path"`
	Phone         *string `json:"phone"`
}

// NewUserResponse ใช้แปลง User จาก database ให้เป็น UserResponse
func NewUserResponse(user *User) UserResponse {
	return UserResponse{
		ID:            user.ID,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Email:         user.Email,
		EmailVerified: user.EmailVerifiedAt != nil,
		HasPassword:   user.PasswordHash != nil,
		PicturePath:   user.PicturePath,
		Phone:         user.Phone,
	}
}
