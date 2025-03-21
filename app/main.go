package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
)

type Ctx struct {
	Client *imapclient.Client
	Mbox   *imap.SelectData
}

func main() {

	ctx := new(Ctx)
	mux := http.NewServeMux()

	mux.HandleFunc("/login", LoginHandler(ctx))
	mux.HandleFunc("/logout", LogoutHandler(ctx))
	mux.HandleFunc("/mail", MailHandler(ctx))
	http.ListenAndServe(":8080", mux)

}

func LoginHandler(ctx *Ctx) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := make(map[string]string)
		// Проверяем метод запроса - для логина принимаем только POST
		if req.Method != http.MethodPost {
			resp["status"] = "Error"
			resp["message"] = "Method not allowed"
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				log.Fatalf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(jsonResp)
			return
		}

		login := req.PostFormValue("login")
		password := req.PostFormValue("password")

		// Проходим авторизацию в почту и возвращаем текст с количеством писем
		client, err := imapclient.DialTLS("imap.rambler.ru:993", nil)
		if err != nil {
			log.Fatalf("failed to dial IMAP server: %v", err)
		}

		if err := client.Login(login, password).Wait(); err != nil {
			log.Fatalf("failed to login: %v", err)
		}
		selectedMbox, err := client.Select("INBOX", nil).Wait()
		if err != nil {
			log.Fatalf("failed to select INBOX: %v", err)
		}

		ctx.Client = client
		ctx.Mbox = selectedMbox
		log.Printf("INBOX contains %v messages", selectedMbox.NumMessages)

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")

		// Формируем ответ
		if err == nil {
			resp["status"] = "Ok"
			resp["message"] = fmt.Sprintf("INBOX contains %v messages", selectedMbox.NumMessages)
		} else {
			resp["status"] = "Error"
			resp["message"] = fmt.Sprintf("ERROR: %v", err)
		}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)
	}
}

func LogoutHandler(ctx *Ctx) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := make(map[string]string)
		// Проверяем, что клиент определен
		if ctx.Client == nil {
			resp["status"] = "Error"
			resp["message"] = "Client is down"
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				log.Fatalf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(jsonResp)
			return
		}
		// Выходим из аккаунта
		err := ctx.Client.Logout().Wait()
		if err != nil {
			log.Fatalf("Logout error. Err: %s", err)
		}
		log.Println("Client logged out :", ctx.Client)
		// Формируем ответ
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")

		if err == nil {
			resp["status"] = "Ok"
			resp["message"] = "Logout successfull"
		} else {
			resp["status"] = "Error"
			resp["message"] = fmt.Sprintf("ERROR: %v", err)
			log.Fatalf("failed to logout: %v", err)
		}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)
	}
}

func MailHandler(ctx *Ctx) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := make(map[string]string)
		// Проверяем, что клиент определен
		if ctx.Client == nil {
			resp["status"] = "Error"
			resp["message"] = "Client is down"
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				log.Fatalf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(jsonResp)
			return
		}
		log.Println("Mail client:", ctx.Client)

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		// Ошибка, если сообщений в почте нет
		if ctx.Mbox.NumMessages == 0 {
			resp["status"] = "Error"
			resp["message"] = "No messages in the mailbox"
			log.Fatalf("No messages in the mailbox")
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				log.Fatalf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(jsonResp)
		} else {
			// Send a FETCH command to fetch the message body
			seqSet := imap.SeqSetNum(1)
			bodySection := &imap.FetchItemBodySection{}
			fetchOptions := &imap.FetchOptions{
				BodySection: []*imap.FetchItemBodySection{bodySection},
			}
			fetchCmd := ctx.Client.Fetch(seqSet, fetchOptions)
			defer fetchCmd.Close()

			msg := fetchCmd.Next()
			if msg == nil {
				log.Fatalf("FETCH command did not return any message")
			}

			// Find the body section in the response
			var bodySectionData imapclient.FetchItemDataBodySection
			ok := false
			for {
				item := msg.Next()
				if item == nil {
					break
				}
				bodySectionData, ok = item.(imapclient.FetchItemDataBodySection)
				if ok {
					break
				}
			}
			if !ok {
				log.Fatalf("FETCH command did not return body section")
			}

			// Read the message via the go-message library
			mr, err := mail.CreateReader(bodySectionData.Literal)
			if err != nil {
				log.Fatalf("failed to create mail reader: %v", err)
			}

			// Print a few header fields
			h := mr.Header
			var subject string
			if date, err := h.Date(); err != nil {
				log.Printf("failed to parse Date header field: %v", err)
			} else {
				log.Printf("Date: %v", date)
			}
			if to, err := h.AddressList("To"); err != nil {
				log.Printf("failed to parse To header field: %v", err)
			} else {
				log.Printf("To: %v", to)
			}
			if subject, err := h.Text("Subject"); err != nil {
				log.Printf("failed to parse Subject header field: %v", err)
			} else {
				log.Printf("Subject: %v", subject)
			}

			// Process the message's parts
			var body string
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					log.Fatalf("failed to read message part: %v", err)
				}

				switch h := p.Header.(type) {
				case *mail.InlineHeader:
					// This is the message's text (can be plain-text or HTML)
					b, _ := io.ReadAll(p.Body)
					body = string(b)
					log.Printf("Inline text: %v", body)
				case *mail.AttachmentHeader:
					// This is an attachment
					filename, _ := h.Filename()
					log.Printf("Attachment: %v", filename)
				}
			}

			if err := fetchCmd.Close(); err != nil {
				log.Fatalf("FETCH command failed: %v", err)
			}

			if err == nil {
				resp["status"] = "Ok"
				resp["subject"] = subject
				resp["body"] = body
			} else {
				resp["status"] = "Error"
				resp["message"] = fmt.Sprintf("ERROR: %v", err)
				log.Fatalf("failed to logout: %v", err)
			}
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				log.Fatalf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(jsonResp)
		}
	}
}
