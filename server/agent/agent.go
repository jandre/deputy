package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hpcloud/tail"
)

type Config struct {
	Log       string
	URL       string
	AuthToken string
	Host      string
}

func GetConfig() *Config {
	url := os.Getenv("DEPUTY_URL")
	auth := os.Getenv("DEPUTY_AUTH_TOKEN")
	host, _ := os.Hostname()

	log := "/var/log/falco/events.log"

	if url == "" {
		url = "https://deputy.ngrok.io/alerts"
	}
	return &Config{URL: url, AuthToken: auth, Host: host, Log: log}
}

var config = GetConfig()

type Message struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Host    string    `json:"host"`
}

func SendMessage(msg string) {
	fmt.Println(msg)

	m := &Message{
		Time:    time.Now(),
		Message: msg,
		Host:    config.Host,
	}

	b, _ := json.Marshal(m)

	req, err := http.NewRequest("POST", config.URL, bytes.NewBuffer(b))
	req.Header.Set("X-DEPUTY-AUTH", config.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Unable to send message: err=%s resp=%s message=%+v\n", err, resp, *m)
	}
	if resp == nil {
		log.Printf("Unable to send message: err=%s resp=%s message=%+v\n", err, resp, *m)
	} else {
		defer resp.Body.Close()
	}
}

func main() {

	end := &tail.SeekInfo{
		Offset: 0,
		Whence: 2,
	}

	t, err := tail.TailFile(config.Log, tail.Config{Follow: true, Location: end})

	if err != nil {
		panic(err)
	}

	for line := range t.Lines {
		SendMessage(line.Text)
	}
}
