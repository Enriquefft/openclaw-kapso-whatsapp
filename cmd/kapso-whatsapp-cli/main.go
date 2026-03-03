package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/preflight"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "send":
		handleSend(os.Args[2:])
	case "groups":
		handleGroups()
	case "status":
		handleStatus()
	case "preflight":
		handlePreflight()
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Kapso.APIKey == "" || cfg.Kapso.PhoneNumberID == "" {
		fmt.Fprintln(os.Stderr, "error: KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
		os.Exit(1)
	}

	client := kapso.NewClient(cfg.Kapso.APIKey, cfg.Kapso.PhoneNumberID)
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

func handleGroups() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Kapso.APIKey == "" || cfg.Kapso.PhoneNumberID == "" {
		fmt.Fprintln(os.Stderr, "error: KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
		os.Exit(1)
	}

	client := kapso.NewClient(cfg.Kapso.APIKey, cfg.Kapso.PhoneNumberID)
	resp, err := client.ListMessages(kapso.ListMessagesParams{
		Direction: "inbound",
		Limit:     100,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching messages: %v\n", err)
		os.Exit(1)
	}

	// Deduplicate: group ID → last sender name.
	groups := make(map[string]string)
	var order []string
	for _, msg := range resp.Data {
		if msg.Kapso == nil || msg.Kapso.Conversation == nil {
			continue
		}
		id := msg.Kapso.Conversation.ID
		if !strings.HasSuffix(id, "@g.us") {
			continue
		}
		if _, seen := groups[id]; !seen {
			order = append(order, id)
		}
		name := msg.From
		if msg.Kapso.ContactName != "" {
			name = msg.Kapso.ContactName
		}
		groups[id] = name
	}

	if len(groups) == 0 {
		fmt.Println("No groups found in recent messages.")
		fmt.Println("Send a message from a WhatsApp group first, then run this command again.")
		return
	}

	fmt.Println("Groups found in recent messages:")
	fmt.Println()
	for _, id := range order {
		fmt.Printf("  %s    (last message from: %s)\n", id, groups[id])
	}

	fmt.Println()
	fmt.Println("Add these to your config.toml:")
	fmt.Println()
	fmt.Print("  [security]\n  group_ids = [")
	for i, id := range order {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%q", id)
	}
	fmt.Println("]")
}

func handleStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	webhookAddr := cfg.Webhook.Addr
	if !strings.Contains(webhookAddr, "://") {
		webhookAddr = "http://localhost" + webhookAddr
	}

	resp, err := http.Get(webhookAddr + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("webhook server: ok")
	} else {
		fmt.Fprintf(os.Stderr, "webhook server: unhealthy (status %d)\n", resp.StatusCode)
		os.Exit(1)
	}
}

func handlePreflight() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL]  Config — %v\n", err)
		os.Exit(1)
	}
	_ = cfg.Validate()

	fmt.Println("Preflight checks:")
	if err := preflight.Run(cfg, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, "\nPreflight failed.")
		os.Exit(1)
	}
	fmt.Println("\nAll checks passed.")
}

func printUsage() {
	fmt.Println(`kapso-whatsapp-cli — Send WhatsApp messages via Kapso API

Commands:
  send --to +NUMBER --text "message"   Send a text message
  groups                                List WhatsApp group IDs from recent messages
  status                                Check webhook server health
  preflight                             Verify config, credentials, and connectivity
  help                                  Show this help

Configuration:
  Config file: ~/.config/kapso-whatsapp/config.toml (or set KAPSO_CONFIG)
  Env vars KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID override config file values.`)
}
