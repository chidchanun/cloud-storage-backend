package email

import (
	"bytes"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
)

type SMTPSender struct {
	host      string
	port      string
	username  string
	password  string
	fromName  string
	fromEmail string
}

func NewSMTPSender(
	host string,
	port string,
	username string,
	password string,
	fromName string,
	fromEmail string,
) *SMTPSender {
	return &SMTPSender{
		host:      strings.TrimSpace(host),
		port:      strings.TrimSpace(port),
		username:  strings.TrimSpace(username),
		password:  password,
		fromName:  strings.TrimSpace(fromName),
		fromEmail: strings.TrimSpace(fromEmail),
	}
}

func (s *SMTPSender) IsConfigured() bool {
	return s.host != "" &&
		s.port != "" &&
		s.username != "" &&
		s.password != "" &&
		s.fromEmail != ""
}

func (s *SMTPSender) SendVerificationEmail(
	toEmail string,
	toName string,
	verificationLink string,
) error {
	if !s.IsConfigured() {
		return nil
	}

	subject := "Verify your AnuCloud email"
	body := verificationEmailHTML(toName, verificationLink)

	message := bytes.Buffer{}
	message.WriteString("From: ")
	message.WriteString(formatAddress(s.fromName, s.fromEmail))
	message.WriteString("\r\n")
	message.WriteString("To: ")
	message.WriteString(formatAddress(toName, toEmail))
	message.WriteString("\r\n")
	message.WriteString("Subject: ")
	message.WriteString(mime.QEncoding.Encode("utf-8", subject))
	message.WriteString("\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	address := net.JoinHostPort(s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	if err := smtp.SendMail(
		address,
		auth,
		s.fromEmail,
		[]string{toEmail},
		message.Bytes(),
	); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}

func formatAddress(name string, email string) string {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)

	if name == "" {
		return "<" + email + ">"
	}

	return mime.QEncoding.Encode("utf-8", name) + " <" + email + ">"
}

func verificationEmailHTML(name string, verificationLink string) string {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`<!doctype html>
<html>
  <body style="margin:0;background:#f8fafc;font-family:Arial,sans-serif;color:#0f172a;">
    <div style="max-width:560px;margin:0 auto;padding:32px;">
      <h1 style="margin:0 0 12px;font-size:24px;">Verify your AnuCloud email</h1>
      <p style="margin:0 0 20px;line-height:1.6;">Hi %s, please confirm this email address to finish creating your AnuCloud account.</p>
      <a href="%s" style="display:inline-block;background:#2563eb;color:#ffffff;text-decoration:none;font-weight:700;padding:12px 18px;border-radius:8px;">Verify email</a>
      <p style="margin:24px 0 0;font-size:13px;line-height:1.6;color:#475569;">This link expires in 24 hours. If you did not create this account, you can ignore this email.</p>
    </div>
  </body>
</html>`, displayName, verificationLink)
}
