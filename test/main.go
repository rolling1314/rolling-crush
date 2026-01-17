package main

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

func main() {
	from := "ld1779494931@163.com"
	to := []string{"ld1779494931@163.com"} // 给自己发一封测试
	password := "JSiygEevMwKu44Xs"         // ⚠️ 换成【新的】应用专用密码

	smtpHost := "smtp.163.com"
	smtpPort := "465"

	message := []byte("Subject: Gmail SMTP Test\r\n" +
		"\r\n" +
		"Hello, this is a test email from Golang.\r\n")

	auth := smtp.PlainAuth("", from, password, smtpHost)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpHost,
	}

	conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, tlsConfig)
	if err != nil {
		panic(err)
	}

	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		panic(err)
	}

	if err = client.Auth(auth); err != nil {
		panic(err)
	}

	if err = client.Mail(from); err != nil {
		panic(err)
	}

	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			panic(err)
		}
	}

	w, err := client.Data()
	if err != nil {
		panic(err)
	}

	_, err = w.Write(message)
	if err != nil {
		panic(err)
	}

	err = w.Close()
	if err != nil {
		panic(err)
	}

	client.Quit()
	fmt.Println("✅ 邮件发送成功")
}
