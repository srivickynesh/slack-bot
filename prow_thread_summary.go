package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type SlackMessage struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTs string `json:"thread_ts,omitempty"`
}

type SlackResponse struct {
	Ok      bool   `json:"ok"`
	Error   string `json:"error"`
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
}

func FetchTextContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching the webpage: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading the webpage content: %w", err)
	}
	return string(bodyBytes), nil
}

// sends a message to the latest thread in the Slack channel
func SendMessageToLatestThread(token, channelID, message string) error {
	historyParams := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     100,
		Inclusive: false,
	}

	api := slack.New(token)
	history, err := api.GetConversationHistory(historyParams)
	if err != nil {
		return fmt.Errorf("error fetching conversation history: %w", err)
	}

	// "YYYY-MM-DD" format
	today := time.Now().Format("2022-01-02")
	var latestThreadTimestamp string
	for _, message := range history.Messages {
		if strings.HasPrefix(message.ThreadTimestamp, today) {
			latestThreadTimestamp = message.ThreadTimestamp
			break
		}
	}

	if latestThreadTimestamp != "" {
		slackMessage := SlackMessage{
			Channel:  channelID,
			Text:     message,
			ThreadTs: latestThreadTimestamp,
		}

		payloadBuf := new(bytes.Buffer)
		json.NewEncoder(payloadBuf).Encode(slackMessage)
		req, _ := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", payloadBuf)

		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending message to Slack: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("error: not ok response back from Slack API: %s", body)
		}

		var respContent SlackResponse
		if err := json.NewDecoder(resp.Body).Decode(&respContent); err != nil {
			return fmt.Errorf("error decoding response from Slack API: %w", err)
		}

		if !respContent.Ok {
			return fmt.Errorf("error from Slack API: %s", respContent.Error)
		}
	} else {
		return fmt.Errorf("no thread found for today")
	}

	return nil
}

func ConstructMessage(content, bodyString string) string {
	var message string
	stateRe := regexp.MustCompile(`Reporting job state '(\w+)'`)
	stateMatch := stateRe.FindStringSubmatch(bodyString)
	if len(stateMatch) == 2 && stateMatch[1] == "succeeded" {
		message = fmt.Sprintf("Reporting job state: %s\n", strings.TrimSpace(stateMatch[1]))
	} else if len(stateMatch) == 2 && stateMatch[1] == "failed" {
		re := regexp.MustCompile(`(?s)(Summarizing.*?Test Suite Failed)`)
		matches := re.FindStringSubmatch(bodyString)
		if matches == nil {
			message = "No Failure Summary found\n"
		} else {
			message = fmt.Sprintf("%s\n", matches[1])
		}
		message += fmt.Sprintf("Reporting job state: %s\n", strings.TrimSpace(stateMatch[1]))
	}

	durationRe := regexp.MustCompile(`Ran for ([\dhms]+)`)
	durationMatch := durationRe.FindStringSubmatch(bodyString)
	message += fmt.Sprintf("Ran for %s\n", durationMatch[1])

	return message
}

func main() {
	url := os.Getenv("URL")
	content, err := FetchTextContent(url)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	prowURL := fmt.Sprintf(os.Getenv("PROW_URL"), content)
	bodyString, err := FetchTextContent(prowURL)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	message := ConstructMessage(content, bodyString)

	token := os.Getenv("SLACK_TOKEN")
	channelID := os.Getenv("CHANNEL_ID")
	err = SendMessageToLatestThread(token, channelID, message)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Slack message sent successfully!")
}
