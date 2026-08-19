package push

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"

	"github.com/MinChen05/JYX-BI/internal/config"
)

// sendEmail 发送带 xlsx 附件的邮件。支持 465(SSL) 与 587(STARTTLS/明文)。
func sendEmail(cfg *config.Config, to, defName string, params map[string]string, xlsx []byte) error {
	if cfg.SMTP.Host == "" {
		return fmt.Errorf("smtp 未配置")
	}
	subject := fmt.Sprintf("【报表】%s %s", defName, paramsString(params))

	var body strings.Builder
	body.WriteString("报表已提交。\n\n")
	for k, v := range params {
		body.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	body.WriteString("\n附件为填报数据。\n")

	var msg bytes.Buffer
	w := multipart.NewWriter(&msg)
	part, err := w.CreatePart(textproto.MIMEHeader{"Content-Type": []string{"text/plain; charset=utf-8"}})
	if err != nil {
		return err
	}
	if _, err := part.Write([]byte(body.String())); err != nil {
		return err
	}
	fh, err := w.CreateFormFile("attachment", defName+".xlsx")
	if err != nil {
		return err
	}
	if _, err := fh.Write(xlsx); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	addr := net.JoinHostPort(cfg.SMTP.Host, fmt.Sprintf("%d", cfg.SMTP.Port))
	mailMsg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n%s",
		cfg.SMTP.From, to, subject, w.Boundary(), msg.String())

	var auth smtp.Auth
	if cfg.SMTP.User != "" {
		auth = smtp.PlainAuth("", cfg.SMTP.User, cfg.SMTP.Pass, cfg.SMTP.Host)
	}

	if cfg.SMTP.Port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SMTP.Host})
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, cfg.SMTP.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
		if err := client.Mail(cfg.SMTP.From); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		wr, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := wr.Write([]byte(mailMsg)); err != nil {
			return err
		}
		_ = wr.Close()
		return client.Quit()
	}
	return smtp.SendMail(addr, auth, cfg.SMTP.From, []string{to}, []byte(mailMsg))
}
