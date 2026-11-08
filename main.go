package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const rateURL = "https://unired-mobile-api.cloudgate.uz/get_rate_marketing/"

type Rate struct {
	Currency string          `json:"currency_name"`
	Sell     json.RawMessage `json:"sell"`
}

type State struct {
	Rate *float64 `json:"rate"`
}

func main() {
	client := &http.Client{Timeout: 30 * time.Second}

	var resp *http.Response
	var err error

	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = client.Get(rateURL)
		if err == nil {
			break
		}

		log.Printf("Request failed (attempt %d/3): %v", attempt, err)
		if attempt < 3 {
			time.Sleep(2 * time.Second)
		}
	}

	check(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("rate API returned %s", resp.Status)
	}

	var rates []Rate
	check(json.NewDecoder(resp.Body).Decode(&rates))

	var current float64
	for _, rate := range rates {
		if rate.Currency == "UZ_TO_RU" {
			check(json.Unmarshal(rate.Sell, &current))
			break
		}
	}
	if current <= 0 {
		log.Fatal("UZ_TO_RU sell rate not found")
	}

	var state State
	data, err := os.ReadFile("state.json")
	if err == nil {
		check(json.Unmarshal(data, &state))
	} else if !os.IsNotExist(err) {
		check(err)
	}

	if state.Rate != nil && *state.Rate == current {
		fmt.Println("Rate unchanged, skipping notification")
		return
	}

	message := fmt.Sprintf("1 RUB = %.2f UZS\nSource: Unired", current)
	if state.Rate != nil {
		change := current - *state.Rate
		prefix := ""
		if change < 0 {
			prefix = "⭐ Rate decreased!\n\n"
		}
		message = fmt.Sprintf("%s1 RUB = %.2f UZS\nPrevious: %.2f UZS\nChange: %+.2f UZS\nSource: Unired",
			prefix, current, *state.Rate, change)
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required")
	}

	form := url.Values{"chat_id": {chatID}, "text": {message}}
	answer, err := client.PostForm("https://api.telegram.org/bot"+token+"/sendMessage", form)
	check(err)
	defer answer.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	check(json.NewDecoder(answer.Body).Decode(&result))
	if !result.OK {
		log.Fatalf("Telegram: %s", result.Description)
	}

	updated, err := json.MarshalIndent(State{Rate: &current}, "", "  ")
	check(err)
	check(os.WriteFile("state.json", append(updated, '\n'), 0644))
	fmt.Println(message)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
