package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/emersion/go-imap/v2/imapclient"
)

func main() {

	var client imapclient.Client
	mux := http.NewServeMux()

	mux.HandleFunc("/login", LoginHandler(&client))
	//mux.HandleFunc("/logout", LogoutHandler(&ctx))
	//mux.HandleFunc("/mail", MailHandler(&ctx))
	http.ListenAndServe(":8080", mux)

}

func LoginHandler(client *imapclient.Client) func(http.ResponseWriter, *http.Request) {
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
		log.Printf("INBOX contains %v messages", selectedMbox.NumMessages)

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")

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

func LogoutHandler(client *imapclient.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := make(map[string]string)
		// Проверяем, что контекст определен
		if client == nil {
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

		err := client.Logout().Wait()

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
