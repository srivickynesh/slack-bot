package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func readParticipantsFromFile(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	var participants []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			participants = append(participants, trimmed)
		}
	}
	return participants, nil
}

func readLastWeekFromFile(filePath string) []string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading last week file: %v\n", err)
		return nil
	}
	return strings.Split(string(content), "\n")
}

func filterEligibleParticipants(participants, lastWeekParticipants []string) []string {
	var eligible []string
	for _, p := range participants {
		if !stringInSlice(p, lastWeekParticipants) {
			eligible = append(eligible, p)
		}
	}
	return eligible
}

func stringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

func writeLastWeekToFile(filePath string, lastWeek []string) error {
	return os.WriteFile(filePath, []byte(strings.Join(lastWeek, "\n")), 0644)
}

func sendMessageToSlack(token, channelID string, users []string) error {
	slackURL := "https://slack.com/api/chat.postMessage"
	currentMonth := time.Now().Month().String()
	for i, user := range users {
		users[i] = "<@" + user + ">"
	}
	message := fmt.Sprintf("Coffee break group for %s is: %s", currentMonth, strings.Join(users, ", "))
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

func main() {
	dirPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	slackToken := os.Getenv("SLACK_TOKEN")
	slackChannelID := os.Getenv("HACBS_CHANNEL_ID")
	participants, err := readParticipantsFromFile(filepath.Join(dirPath, "coffee-break/participants.txt"))
	if err != nil {
		log.Fatalf("Error reading participants file: %v\n", err)
	}
	if len(participants) < 3 {
		log.Fatalf("Not enough participants to form a group\n")
	}
	lastWeek := readLastWeekFromFile(filepath.Join(dirPath, "coffee-break/last_week.txt"))
	lastWeekParticipants := strings.Split(lastWeek[len(lastWeek)-1], ", ")
	eligibleParticipants := filterEligibleParticipants(participants, lastWeekParticipants)
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	r.Shuffle(len(eligibleParticipants), func(i, j int) {
		eligibleParticipants[i], eligibleParticipants[j] = eligibleParticipants[j], eligibleParticipants[i]
	})
	newGroup := eligibleParticipants[:3]
	lastWeek = append(lastWeek, strings.Join(newGroup, ", "))
	err = writeLastWeekToFile(filepath.Join(dirPath, "coffee-break/last_week.txt"), lastWeek)
	if err != nil {
		log.Fatalf("Error writing to last week file: %v\n", err)
	}
	err = sendMessageToSlack(slackToken, slackChannelID, newGroup)
	if err != nil {
		log.Fatalf("Error sending message to Slack: %v\n", err)
	}
}
