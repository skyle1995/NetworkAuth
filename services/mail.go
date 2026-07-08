package services

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// ============================================================================
// SMTP 邮件发送
// ============================================================================
//
// 从系统设置读取 SMTP 配置，支持三种连接方式：
//   - SSL/隐式TLS（465）：直接 TLS 拨号
//   - STARTTLS（587）：明文拨号后升级 TLS
//   - 明文（25）：不加密（不推荐）

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	SSL      bool
	Username string
	Password string
	From     string
	FromName string
}

// GetSMTPConfig 从系统设置读取 SMTP 配置
func GetSMTPConfig() SMTPConfig {
	s := GetSettingsService()
	return SMTPConfig{
		Enabled:  s.GetBool("smtp_enabled", false),
		Host:     s.GetString("smtp_host", ""),
		Port:     s.GetInt("smtp_port", 465),
		SSL:      s.GetBool("smtp_ssl", true),
		Username: s.GetString("smtp_username", ""),
		Password: s.GetString("smtp_password", ""),
		From:     s.GetString("smtp_from", ""),
		FromName: s.GetString("smtp_from_name", "NetworkAuth"),
	}
}

// validate 校验配置是否可用于发信
func (cfg SMTPConfig) validate() error {
	if !cfg.Enabled {
		return errors.New("邮件服务未开启")
	}
	if cfg.Host == "" || cfg.Port == 0 {
		return errors.New("SMTP服务器未配置")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return errors.New("SMTP账号或密码未配置")
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	if from == "" {
		return errors.New("发件人未配置")
	}
	return nil
}

// buildMessage 构造符合 RFC 的邮件报文（UTF-8 HTML 正文）
func buildMessage(cfg SMTPConfig, to, subject, htmlBody string) []byte {
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	fromHeader := from
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("UTF-8", cfg.FromName), from)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.BEncoding.Encode("UTF-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

// SendMail 按系统 SMTP 配置发送一封 HTML 邮件
func SendMail(to, subject, htmlBody string) error {
	cfg := GetSMTPConfig()
	if err := cfg.validate(); err != nil {
		return err
	}

	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	msg := buildMessage(cfg, to, subject, htmlBody)
	tlsCfg := &tls.Config{ServerName: cfg.Host}

	var client *smtp.Client
	var err error
	if cfg.SSL {
		// 隐式 TLS（465）
		conn, derr := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
		if derr != nil {
			return fmt.Errorf("连接SMTP失败: %w", derr)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
	} else {
		// 明文拨号，若支持则升级 STARTTLS
		conn, derr := net.DialTimeout("tcp", addr, 10*time.Second)
		if derr != nil {
			return fmt.Errorf("连接SMTP失败: %w", derr)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err == nil {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if terr := client.StartTLS(tlsCfg); terr != nil {
					client.Close()
					return fmt.Errorf("STARTTLS失败: %w", terr)
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("建立SMTP会话失败: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("发送数据失败: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("写入邮件失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("关闭数据流失败: %w", err)
	}
	return client.Quit()
}
