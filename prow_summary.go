package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type SlackMessage struct {
	Text string `json:"text"`
}

func RemoveANSIEscapeSequences(text string) string {
	regex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return regex.ReplaceAllString(text, "")
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

func SendMessageToLatestThread(token, channelID, message string) error {
	slackURL := "https://slack.com/api/chat.postMessage"

	payload := url.Values{}
	payload.Set("channel", channelID)
	payload.Set("text", message)

	req, err := http.NewRequest("POST", slackURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return fmt.Errorf("error creating the request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending the request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func ConstructMessage(content, bodyString string) (string, bool) {
	var message string
	stateRe := regexp.MustCompile(`Reporting job state '(\w+)'`)
	stateMatch := stateRe.FindStringSubmatch(bodyString)
	if len(stateMatch) == 2 && stateMatch[1] == "failed" {
		re := regexp.MustCompile(`(?s)(Summarizing.*?Test Suite Failed)`)
		matches := re.FindStringSubmatch(bodyString)
		if matches == nil {
			return "", false
		}
		// Remove ANSI escape sequences from the failure summary
		failureSummary := RemoveANSIEscapeSequences(matches[1])
		message = fmt.Sprintf("%s", failureSummary)
		message += fmt.Sprintf("Reporting job state: %s\n", strings.TrimSpace(stateMatch[1]))

		durationRe := regexp.MustCompile(`Ran for ([\dhms]+)`)
		durationMatch := durationRe.FindStringSubmatch(bodyString)
		message += fmt.Sprintf("\nRan for %s\n", durationMatch[1])
		return message, true
	}
	return "", false
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

	message, sendSlackMessage := ConstructMessage(content, bodyString)

	if sendSlackMessage {
		token := os.Getenv("SLACK_TOKEN")
		channelID := os.Getenv("CHANNEL_ID")
		err = SendMessageToLatestThread(token, channelID, message)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("Slack message sent successfully!")
	} else {
		fmt.Println("No test failures found. Slack message not sent.")
	}
}
