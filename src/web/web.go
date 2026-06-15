package web

import (
	"bytes"
	"car_organizer_bot/src/database"
	"car_organizer_bot/src/models"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
)

type WebServer struct {
	DB            *database.Database
	SlackClient   *slack.Client
	Port          string
	SigningSecret string
}

func NewWebServer(db *database.Database, slackToken, signingSecret, port string) *WebServer {
	return &WebServer{
		DB:            db,
		SlackClient:   slack.New(slackToken),
		SigningSecret: signingSecret,
		Port:          port,
	}
}

func (s *WebServer) Start() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Nothing to do here")
	})

	router.POST("/", s.verifyRequest, s.HandleSlashCommand)
	router.POST("/webhook", s.verifyRequest, s.HandleInteractive)

	log.Printf("Slack HTTP server starting on port %s", s.Port)
	if err := router.Run(":" + s.Port); err != nil {
		log.Fatalf("failed to start slack HTTP server: %v", err)
	}
}

func (s *WebServer) verifyRequest(c *gin.Context) {
	if s.SigningSecret == "" {
		c.Next()
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	timestamp := c.GetHeader("X-Slack-Request-Timestamp")
	signature := c.GetHeader("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Slack headers"})
		return
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid timestamp"})
		return
	}

	if abs(time.Now().Unix()-ts) > 300 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "request expired"})
		return
	}

	base := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(s.SigningSecret))
	mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	c.Next()
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (s *WebServer) HandleSlashCommand(c *gin.Context) {
	command := c.PostForm("command")
	channelID := c.PostForm("channel_id")
	userID := c.PostForm("user_id")
	text := strings.TrimSpace(c.PostForm("text"))

	switch command {
	case "/trip":
		s.createTrip(c, channelID, text)
	case "/seats":
		s.updateSeats(c, channelID, userID, text)
	case "/name":
		s.handleName(c, userID, text)
	default:
		c.String(http.StatusOK, "Command not found")
	}
}

func (s *WebServer) HandleInteractive(c *gin.Context) {
	payloadStr := c.PostForm("payload")
	if payloadStr == "" {
		c.String(http.StatusBadRequest, "missing payload")
		return
	}

	var payload slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		log.Printf("failed to parse interaction payload: %v", err)
		c.String(http.StatusBadRequest, "invalid payload")
		return
	}

	var value string
	if len(payload.ActionCallback.BlockActions) > 0 {
		value = payload.ActionCallback.BlockActions[0].Value
	} else if len(payload.ActionCallback.AttachmentActions) > 0 {
		value = payload.ActionCallback.AttachmentActions[0].Value
	} else {
		c.String(http.StatusBadRequest, "no actions")
		return
	}

	switch {
	case strings.HasPrefix(value, "add_car_"):
		s.addCar(payload, c)
	case strings.HasPrefix(value, "join_"):
		s.joinCar(payload, c)
	case strings.HasPrefix(value, "leave_"):
		s.leaveTrip(payload, c)
	default:
		c.String(http.StatusOK, "unknown action")
	}
}

func buildTripBlocks(tripID int64, tripText string, cars []models.CarButton) []slack.Block {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", tripText, false, false),
			nil, nil,
		),
	}

	var elements []slack.BlockElement
	for _, car := range cars {
		elements = append(elements, slack.NewButtonBlockElement(
			"join",
			fmt.Sprintf("join_%d", car.ID),
			slack.NewTextBlockObject("plain_text", fmt.Sprintf("Join %s", car.Name), false, false),
		))
	}

	elements = append(elements, slack.NewButtonBlockElement(
		"add_car",
		fmt.Sprintf("add_car_%d", tripID),
		slack.NewTextBlockObject("plain_text", "Add 🚙", false, false),
	))

	elements = append(elements, slack.NewButtonBlockElement(
		"leave",
		fmt.Sprintf("leave_%d", tripID),
		slack.NewTextBlockObject("plain_text", "Leave trip", false, false),
	))

	blocks = append(blocks, slack.NewActionBlock("actions", elements...))
	return blocks
}

func (s *WebServer) createTrip(c *gin.Context, channelID, tripName string) {
	if tripName == "" {
		c.String(http.StatusOK, "Please provide a trip name: /trip [name]")
		return
	}

	tripID, err := s.DB.InsertTrip(channelID, tripName)
	if err != nil {
		log.Printf("failed to insert trip: %v", err)
		c.String(http.StatusOK, "Unable to create trip")
		return
	}

	c.String(http.StatusOK, "")

	tripText := fmt.Sprintf("📆 *%s*\n\nAdd a new 🚙 or jump in", tripName)
	_, timestamp, err := s.SlackClient.PostMessage(
		channelID,
		slack.MsgOptionBlocks(buildTripBlocks(tripID, tripText, nil)...),
		slack.MsgOptionText(tripText, false),
	)
	if err != nil {
		log.Printf("failed to post slack message: %v", err)
		return
	}

	if err := s.DB.UpdateTripMessageID(tripID, timestamp); err != nil {
		log.Printf("failed to update trip message id: %v", err)
	}
}

func (s *WebServer) addCar(payload slack.InteractionCallback, c *gin.Context) {
	var value string
	if len(payload.ActionCallback.BlockActions) > 0 {
		value = payload.ActionCallback.BlockActions[0].Value
	} else {
		value = payload.ActionCallback.AttachmentActions[0].Value
	}

	tripIDStr := strings.TrimPrefix(value, "add_car_")
	tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusOK, "Invalid trip")
		return
	}

	channelID := payload.Channel.ID
	userID := payload.User.ID
	userName := payload.User.Name

	trip, err := s.DB.SelectTripByID(tripID)
	if err != nil {
		log.Printf("trip not found: %v", err)
		c.String(http.StatusOK, "Trip not found")
		return
	}
	if trip.ChatID != channelID {
		c.String(http.StatusOK, "Trip not found in this channel")
		return
	}

	if err := s.DB.AddCar(tripID, userID, userName); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			c.JSON(http.StatusOK, map[string]string{
				"text":          "You already have a car in this trip!",
				"response_type": "ephemeral",
			})
			return
		}
		log.Printf("failed to add car: %v", err)
		c.JSON(http.StatusOK, map[string]string{
			"text":          "Failed to add car",
			"response_type": "ephemeral",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"text":          "You have added your car! Use the Leave trip button to remove it.",
		"response_type": "ephemeral",
	})

	s.updateTripMessage(channelID, tripID)
}

func (s *WebServer) joinCar(payload slack.InteractionCallback, c *gin.Context) {
	var value string
	if len(payload.ActionCallback.BlockActions) > 0 {
		value = payload.ActionCallback.BlockActions[0].Value
	} else {
		value = payload.ActionCallback.AttachmentActions[0].Value
	}

	carIDStr := strings.TrimPrefix(value, "join_")
	carID, err := strconv.ParseInt(carIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusOK, "Invalid car")
		return
	}

	channelID := payload.Channel.ID
	userID := payload.User.ID
	userName := payload.User.Name

	tripID, err := s.DB.AddOrMovePassenger(carID, userID, userName)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			c.JSON(http.StatusOK, map[string]string{
				"text":          "You are already in that car!",
				"response_type": "ephemeral",
			})
			return
		}
		log.Printf("failed to join car: %v", err)
		c.JSON(http.StatusOK, map[string]string{
			"text":          "Failed to join car!",
			"response_type": "ephemeral",
		})
		return
	}

	c.String(http.StatusOK, "")

	s.updateTripMessage(channelID, tripID)
}

func (s *WebServer) updateSeats(c *gin.Context, channelID, userID, text string) {
	maxPassengers, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		c.String(http.StatusOK, "Usage: /seats [number]")
		return
	}

	tripID, err := s.DB.UpdateCarSeats(channelID, userID, maxPassengers)
	if err != nil {
		log.Printf("failed to update seats: %v", err)
		c.String(http.StatusOK, "Operation not completed, no car found.")
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"text":          "Updated your car seats!",
		"response_type": "ephemeral",
	})

	s.updateTripMessage(channelID, tripID)
}

func (s *WebServer) leaveTrip(payload slack.InteractionCallback, c *gin.Context) {
	var value string
	if len(payload.ActionCallback.BlockActions) > 0 {
		value = payload.ActionCallback.BlockActions[0].Value
	} else {
		value = payload.ActionCallback.AttachmentActions[0].Value
	}

	tripIDStr := strings.TrimPrefix(value, "leave_")
	tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusOK, "Invalid trip")
		return
	}

	channelID := payload.Channel.ID
	userID := payload.User.ID

	if err := s.DB.LeaveTrip(tripID, userID); err != nil {
		log.Printf("failed to leave trip: %v", err)
		c.JSON(http.StatusOK, map[string]string{
			"text":          "Failed to leave trip",
			"response_type": "ephemeral",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"text":          "You left the trip!",
		"response_type": "ephemeral",
	})

	s.updateTripMessage(channelID, tripID)
}

func (s *WebServer) handleName(c *gin.Context, userID, text string) {
	if text == "" {
		c.String(http.StatusOK, "Usage: /name [your display name]")
		return
	}

	if err := s.DB.UpdateName(userID, text); err != nil {
		log.Printf("failed to update name: %v", err)
		c.JSON(http.StatusOK, map[string]string{
			"text":          "Failed to update name",
			"response_type": "ephemeral",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"text":          fmt.Sprintf("Display name updated to %s", text),
		"response_type": "ephemeral",
	})

	trips, err := s.DB.GetTripsByUserID(userID)
	if err != nil {
		log.Printf("failed to get user trips: %v", err)
		return
	}
	for _, trip := range trips {
		if trip.MessageID != nil {
			s.updateTripMessage(trip.ChatID, trip.ID)
		}
	}
}

func (s *WebServer) updateTripMessage(channelID string, tripID int64) {
	trip, err := s.DB.SelectTripByID(tripID)
	if err != nil {
		log.Printf("failed to get trip: %v", err)
		return
	}
	if trip.MessageID == nil {
		return
	}

	text, err := s.DB.BuildTripMessage(tripID)
	if err != nil {
		log.Printf("failed to build trip message: %v", err)
		return
	}
	text = htmlToMrkdwn(text)

	cars, err := s.DB.GetCarsByTrip(tripID)
	if err != nil {
		log.Printf("failed to get cars: %v", err)
		return
	}

	blocks := buildTripBlocks(tripID, text, cars)

	_, _, _, err = s.SlackClient.UpdateMessage(
		channelID,
		*trip.MessageID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		log.Printf("failed to update slack message: %v", err)
	}
}

func htmlToMrkdwn(html string) string {
	result := html
	result = strings.ReplaceAll(result, "<b>", "*")
	result = strings.ReplaceAll(result, "</b>", "*")
	result = strings.ReplaceAll(result, "<i>", "_")
	result = strings.ReplaceAll(result, "</i>", "_")
	result = strings.ReplaceAll(result, "<code>", "`")
	result = strings.ReplaceAll(result, "</code>", "`")
	return result
}
