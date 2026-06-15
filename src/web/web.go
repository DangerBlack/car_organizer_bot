package web

import (
	"car_organizer_bot/src/database"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
)

type WebServer struct {
	DB          *database.Database
	SlackClient *slack.Client
	Port        string
}

func NewWebServer(db *database.Database, slackToken, port string) *WebServer {
	return &WebServer{
		DB:          db,
		SlackClient: slack.New(slackToken),
		Port:        port,
	}
}

func (s *WebServer) Start() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Nothing to do here")
	})

	router.POST("/", s.HandleSlashCommand)
	router.POST("/webhook", s.HandleInteractive)

	log.Printf("Slack HTTP server starting on port %s", s.Port)
	if err := router.Run(":" + s.Port); err != nil {
		log.Fatalf("failed to start slack HTTP server: %v", err)
	}
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
	case "/delete":
		s.deleteCar(c, channelID, userID)
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

	if len(payload.ActionCallback.AttachmentActions) == 0 {
		c.String(http.StatusBadRequest, "no actions")
		return
	}

	action := payload.ActionCallback.AttachmentActions[0]
	value := action.Value

	switch {
	case strings.HasPrefix(value, "add_car"):
		s.addCar(payload, c)
	case strings.HasPrefix(value, "join_"):
		s.joinCar(payload, c)
	default:
		c.String(http.StatusOK, "unknown action")
	}
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

	// Acknowledge first
	c.String(http.StatusOK, "")

	tripIDStr := fmt.Sprintf("%d", tripID)
	_, timestamp, err := s.SlackClient.PostMessage(
		channelID,
		slack.MsgOptionText(fmt.Sprintf("📆 *%s*", tripName), false),
		slack.MsgOptionAttachments(slack.Attachment{
			Text:        "Add a new 🚙 or jump in",
			Fallback:    "You are unable to add a car",
			CallbackID:  fmt.Sprintf("add_car_%d", tripID),
			Color:       "#3AA3E3",
			Actions: []slack.AttachmentAction{
				{
					Name:  "add car",
					Text:  "Add 🚙",
					Type:  "button",
					Value: fmt.Sprintf("add_car_%s", tripIDStr),
				},
			},
		}),
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
	value := payload.ActionCallback.AttachmentActions[0].Value
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

	// Acknowledge the button press
	c.JSON(http.StatusOK, map[string]string{
		"text":          "You have added your car! Remove it with /delete",
		"response_type": "ephemeral",
	})

	s.updateTripMessage(channelID, tripID)
}

func (s *WebServer) joinCar(payload slack.InteractionCallback, c *gin.Context) {
	value := payload.ActionCallback.AttachmentActions[0].Value
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

	// Acknowledge
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

func (s *WebServer) deleteCar(c *gin.Context, channelID, userID string) {
	tripID, _, err := s.DB.UserHasCarInChat(channelID, userID)
	if err != nil {
		c.String(http.StatusOK, "Operation not completed, no car found.")
		return
	}

	if err := s.DB.RemoveCar(tripID, userID); err != nil {
		log.Printf("failed to remove car: %v", err)
		c.String(http.StatusOK, "Operation not completed for unexpected reason!")
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"text":          "Your car has been removed!",
		"response_type": "ephemeral",
	})

	s.updateTripMessage(channelID, tripID)
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

	cars, err := s.DB.GetCarsByTrip(tripID)
	if err != nil {
		log.Printf("failed to get cars: %v", err)
		return
	}

	actions := make([]slack.AttachmentAction, 0, len(cars)+1)
	for _, car := range cars {
		actions = append(actions, slack.AttachmentAction{
			Name:  fmt.Sprintf("Join %s", car.Name),
			Text:  fmt.Sprintf("Join %s", car.Name),
			Type:  "button",
			Value: fmt.Sprintf("join_%d", car.ID),
		})
	}
	actions = append(actions, slack.AttachmentAction{
		Name:  "add car",
		Text:  "Add 🚙",
		Type:  "button",
		Value: fmt.Sprintf("add_car_%d", tripID),
	})

	_, _, _, err = s.SlackClient.UpdateMessage(
		channelID,
		*trip.MessageID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionAttachments(slack.Attachment{
			Text:       "Add a new 🚙 or jump in",
			Fallback:   "You are unable to add a car",
			CallbackID: fmt.Sprintf("add_car_%d", tripID),
			Color:      "#3AA3E3",
			Actions:    actions,
		}),
	)
	if err != nil {
		log.Printf("failed to update slack message: %v", err)
	}
}
