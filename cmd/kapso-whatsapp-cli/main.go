package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "send":
		handleSend(os.Args[2:])
	case "status":
		handleStatus()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleSend(args []string) {
	var to, text string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--to":
			if i+1 < len(args) {
				to = args[i+1]
				i++
			}
		case "--text":
			if i+1 < len(args) {
				text = args[i+1]
				i++
			}
		default:
			// Allow positional: send +NUMBER "message"
			if to == "" && strings.HasPrefix(args[i], "+") {
				to = args[i]
			} else if text == "" {
				text = args[i]
			}
		}
	}

	if to == "" || text == "" {
		fmt.Fprintln(os.Stderr, "usage: kapso-whatsapp-cli send --to +NUMBER --text \"message\"")
		os.Exit(1)
	}

	apiKey := os.Getenv("KAPSO_API_KEY")
	phoneNumberID := os.Getenv("KAPSO_PHONE_NUMBER_ID")

	if apiKey == "" || phoneNumberID == "" {
		fmt.Fprintln(os.Stderr, "error: KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
		os.Exit(1)
	}

	client := kapso.NewClient(apiKey, phoneNumberID)
	resp, err := client.SendText(to, text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Messages) > 0 {
		fmt.Printf("sent (id: %s)\n", resp.Messages[0].ID)
	} else {
		fmt.Println("sent")
	}
}

func handleStatus() {
	webhookAddr := os.Getenv("KAPSO_WEBHOOK_ADDR")
	if webhookAddr == "" {
		webhookAddr = "http://localhost:18790"
	}

	resp, err := http.Get(webhookAddr + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("webhook server: ok")
	} else {
		fmt.Fprintf(os.Stderr, "webhook server: unhealthy (status %d)\n", resp.StatusCode)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`kapso-whatsapp-cli — Send WhatsApp messages via Kapso API

Commands:
  send --to +NUMBER --text "message"   Send a text message
  status                                Check webhook server health
  help                                  Show this help

Environment:
  KAPSO_API_KEY           Kapso API key (required for send)
  KAPSO_PHONE_NUMBER_ID   Kapso phone number ID (required for send)
  KAPSO_WEBHOOK_ADDR      Webhook server address (default: http://localhost:18790)`)
}
