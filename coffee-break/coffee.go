package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

func main() {
	dirPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// 	message := `The FIRST PERSON in the list for each group is responsible for scheduling.
	// If there are conflicts and it is difficult to schedule, this is a communication opportunity.
	// Msg the other people, explain the situation and ask if anything can be moved.
	// If one person isn't available, feel free to have the coffee break with the other person.`

	currentMonth := time.Now().Month().String()

	slackToken := os.Getenv("SLACK_TOKEN")
	slackChannelID := os.Getenv("HACBS_CHANNEL_ID")

	participantsContent, err := ioutil.ReadFile(filepath.Join(dirPath, "coffee-break/participants.txt"))
	if err != nil {
		log.Fatalf("Error reading participants file: %v\n", err)
	}

	participantEntries := strings.Split(string(participantsContent), "\n")
	var participants []string
	for _, participant := range participantEntries {
		trimmed := strings.TrimSpace(participant)
		if trimmed != "" {
			participants = append(participants, trimmed)
		}
	}

	if len(participants) < 3 {
		log.Fatalf("Not enough participants to form a group\n")
	}

	lastWeekContent, err := ioutil.ReadFile(filepath.Join(dirPath, "coffee-break/last_week.txt"))
	if err != nil {
		log.Fatalf("Error reading last week file: %v\n", err)
	}

	lastWeek := strings.Split(string(lastWeekContent), "\n")

	if len(lastWeek) > 6 {
		lastWeek = lastWeek[len(lastWeek)-6:]
	}

	lastWeekParticipants := strings.Split(lastWeek[len(lastWeek)-1], ", ")
	var eligibleParticipants []string
	for _, participant := range participants {
		isInLastWeek := false
		for _, lastWeekParticipant := range lastWeekParticipants {
			if participant == lastWeekParticipant {
				isInLastWeek = true
				break
			}
		}
		if !isInLastWeek {
			eligibleParticipants = append(eligibleParticipants, participant)
		}
	}

	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	r.Shuffle(len(eligibleParticipants), func(i, j int) {
		eligibleParticipants[i], eligibleParticipants[j] = eligibleParticipants[j], eligibleParticipants[i]
	})

	newGroup := eligibleParticipants[:3]

	lastWeek = append(lastWeek, strings.Join(newGroup, ", "))
	if len(lastWeek) > 6 {
		lastWeek = lastWeek[len(lastWeek)-6:]
	}

	err = ioutil.WriteFile(filepath.Join(dirPath, "coffee-break/last_week.txt"), []byte(strings.Join(lastWeek, "\n")), 0644)
	if err != nil {
		log.Fatalf("Error writing to last week file: %v\n", err)
	}

	// groupMessage := fmt.Sprintf("%s\nCoffee break group for %s is: %s", message, currentMonth, strings.Join(newGroup, ", "))
	groupMessage := fmt.Sprintf("\nCoffee break group for %s is: %s", currentMonth, strings.Join(newGroup, ", "))
	err = SendMessageToLatestThread(slackToken, slackChannelID, groupMessage)
	if err != nil {
		log.Fatalf("Error sending message to Slack: %v\n", err)
	}
}
