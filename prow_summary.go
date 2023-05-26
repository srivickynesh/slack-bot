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
    result := regex.ReplaceAllString(text, "")

    // Additional debug logs
    if text != result {
        log.Printf("Removed ANSI escape sequences: before [%v] after [%v]", text, result)
    } else {
        log.Printf("No ANSI escape sequences found: [%v]", text)
    }

    return result
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
	const statePattern = `Reporting job state '(\w+)'`
	const failurePattern = `(?s)(Summarizing.*?Test Suite Failed)`
	const durationPattern = `Ran for ([\dhms]+)`

	stateRegexp := regexp.MustCompile(statePattern)
	stateMatches := stateRegexp.FindStringSubmatch(bodyString)

	hasFailed := len(stateMatches) == 2 && stateMatches[1] == "failed"
	if !hasFailed {
		return "", false
	}

	failureRegexp := regexp.MustCompile(failurePattern)
	failureMatches := failureRegexp.FindStringSubmatch(bodyString)
	if failureMatches == nil {
		return "", false
	}

	var failureSummary strings.Builder
	for _, submatch := range failureMatches {
		cleanSubmatch := RemoveANSIEscapeSequences(submatch)
		failureSummary.WriteString(cleanSubmatch)
	}

	message = fmt.Sprintf("%s\n", failureSummary.String())
	message += fmt.Sprintf("Reporting job state: %s\n", strings.TrimSpace(stateMatches[1]))

	durationRegexp := regexp.MustCompile(durationPattern)
	durationMatches := durationRegexp.FindStringSubmatch(bodyString)

	if len(durationMatches) >= 2 {
		message += fmt.Sprintf("Ran for %s\n", durationMatches[1])
	}

	return message, true
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
