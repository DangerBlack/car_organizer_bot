package telegram

import (
	"car_organizer_bot/src/database"
	"car_organizer_bot/src/models"
	"fmt"
	"log"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"
)

type Telegram struct {
	Bot *telebot.Bot
	DB  *database.Database
}

func (t Telegram) SetupHandlers() {
	t.Bot.Handle("/start", t.Start)
	t.Bot.Handle("/help", t.Help)
	t.Bot.Handle("/trip", t.CreateTrip)
	t.Bot.Handle("/seats", t.SetSeats)
	t.Bot.Handle("/remove", t.RemoveCar)
	t.Bot.Handle("/name", t.SetName)

	t.Bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := c.Callback().Data
		log.Printf("callback: %s", data)

		switch {
		case strings.HasPrefix(data, "add_car_"):
			return t.CallbackAddCar(c)
		case strings.HasPrefix(data, "join_"):
			return t.CallbackJoinCar(c)
		default:
			return c.Respond(&telebot.CallbackResponse{Text: "unknown action"})
		}
	})
}

func DefineUsername(user *telebot.User) string {
	if user.Username != "" {
		return user.Username
	}
	if user.FirstName != "" && user.LastName != "" {
		return fmt.Sprintf("%s.%s", user.FirstName[:1], user.LastName)
	}
	if user.FirstName != "" {
		return user.FirstName
	}
	return fmt.Sprintf("ID:%d", user.ID)
}

func (t Telegram) Start(c telebot.Context) error {
	return c.Send("Hello I'm car organizer bot!")
}

func (t Telegram) Help(c telebot.Context) error {
	return c.Send(`Hello I'm car organizer bot!
I'm here to help you organize an easy trip with your friends!

Steps:
    1. Add @car_organizer_bot to your group of friends
    2. Write <code>/trip name_of_the_trip</code> in the group
    3. Click on the Add Car button to make your car available for your friends
    4. Click on the car of a friends if you want jump in
    5. You can customize the number of seats by typing <code>/seats 4</code>
    6. When every member are in a car you are ready to go!`, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

func (t Telegram) CreateTrip(c telebot.Context) error {
	chatID := c.Chat().ID
	args := c.Args()

	if len(args) < 1 {
		return c.Send("Please send the command as /trip [name of the trip]")
	}

	tripName := strings.Join(args, " ")

	tripID, err := t.DB.InsertTrip(chatID, tripName)
	if err != nil {
		log.Printf("failed to insert trip: %v", err)
		return c.Send("Operation not completed for unexpected reason!")
	}

	opts := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
		ReplyMarkup: &telebot.ReplyMarkup{
			InlineKeyboard: [][]telebot.InlineButton{
				{
					{Text: "Add 🚙", Data: fmt.Sprintf("add_car_%d", tripID)},
				},
			},
		},
	}

	msg, err := t.Bot.Send(c.Chat(), fmt.Sprintf("📆 <b>%s</b>", tripName), opts)
	if err != nil {
		log.Printf("failed to send message: %v", err)
		return c.Send("Operation not completed for unexpected reason!")
	}

	if err := t.DB.UpdateTripMessageID(tripID, int64(msg.ID)); err != nil {
		log.Printf("failed to update message id: %v", err)
	}

	return nil
}

func (t Telegram) SetSeats(c telebot.Context) error {
	chatID := c.Chat().ID
	userID := c.Sender().ID
	args := c.Args()

	if len(args) < 1 {
		return c.Send("Usage: /seats [number]")
	}

	maxPassengers, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Send("Please provide a valid number")
	}

	tripID, err := t.DB.UpdateCarSeats(chatID, userID, maxPassengers)
	if err != nil {
		log.Printf("failed to update seats: %v", err)
		if err.Error() == "sql: no rows in result set" {
			return c.Send("Operation not completed, no car found.")
		}
		return c.Send("Operation not completed for unexpected reason!")
	}

	return t.updateTripMessage(c, tripID)
}

func (t Telegram) RemoveCar(c telebot.Context) error {
	chatID := c.Chat().ID
	userID := c.Sender().ID

	tripID, _, err := t.DB.UserHasCarInChat(chatID, userID)
	if err != nil {
		return c.Send("Operation not completed, no car found.")
	}

	if err := t.DB.RemoveCar(tripID, userID); err != nil {
		log.Printf("failed to remove car: %v", err)
		return c.Send("Operation not completed for unexpected reason!")
	}

	return t.updateTripMessage(c, tripID)
}

func (t Telegram) SetName(c telebot.Context) error {
	userID := c.Sender().ID
	args := c.Args()

	if len(args) < 1 {
		return c.Send("Usage: /name [your name]")
	}

	name := strings.Join(args, " ")

	if err := t.DB.UpdateName(userID, name); err != nil {
		log.Printf("failed to update name: %v", err)
		return c.Send("Operation not completed for unexpected reason!")
	}

	trips, err := t.DB.GetTripsByUserID(userID)
	if err != nil {
		log.Printf("failed to get user trips: %v", err)
	} else {
		for _, trip := range trips {
			if trip.MessageID != nil {
				t.editTripMessage(trip.ChatID, *trip.MessageID, trip.ID)
			}
		}
	}

	return c.Send(fmt.Sprintf("Ok I've update your name in every trip to %s", name))
}

func (t Telegram) CallbackAddCar(c telebot.Context) error {
	chatID := c.Chat().ID
	userID := c.Sender().ID
	username := DefineUsername(c.Sender())
	data := c.Callback().Data

	tripIDStr := strings.TrimPrefix(data, "add_car_")
	tripID, err := strconv.ParseInt(tripIDStr, 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid trip"})
	}

	trip, err := t.DB.SelectTripByID(tripID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Trip not found"})
	}
	if trip.ChatID != chatID {
		return c.Respond(&telebot.CallbackResponse{Text: "Trip not found in this chat"})
	}

	if err := t.DB.AddCar(tripID, userID, username); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return c.Respond(&telebot.CallbackResponse{Text: "You already have a car in this trip!"})
		}
		log.Printf("failed to add car: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to add car"})
	}

	return t.editTripCallback(c, tripID)
}

func (t Telegram) CallbackJoinCar(c telebot.Context) error {
	userID := c.Sender().ID
	username := DefineUsername(c.Sender())
	data := c.Callback().Data

	carIDStr := strings.TrimPrefix(data, "join_")
	carID, err := strconv.ParseInt(carIDStr, 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid car"})
	}

	tripID, err := t.DB.AddOrMovePassenger(carID, userID, username)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return c.Respond(&telebot.CallbackResponse{Text: "You are already in that car!"})
		}
		log.Printf("failed to add passenger: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to join car!"})
	}

	return t.editTripCallback(c, tripID)
}

func (t Telegram) updateTripMessage(c telebot.Context, tripID int64) error {
	trip, err := t.DB.SelectTripByID(tripID)
	if err != nil {
		log.Printf("failed to get trip: %v", err)
		return c.Send("Operation not completed for unexpected reason!")
	}

	if trip.MessageID == nil {
		return c.Send("Operation not completed, no message found.")
	}

	return t.editTripMessage(trip.ChatID, *trip.MessageID, tripID)
}

func (t Telegram) editTripMessage(chatID int64, messageID int64, tripID int64) error {
	text, err := t.DB.BuildTripMessage(tripID)
	if err != nil {
		return err
	}

	cars, err := t.DB.GetCarsByTrip(tripID)
	if err != nil {
		return err
	}

	keyboard := buildKeyboard(cars, tripID)

	_, err = t.Bot.Edit(
		&telebot.Message{
			Chat: &telebot.Chat{ID: chatID},
			ID:   int(messageID),
		},
		text,
		&telebot.SendOptions{
			ParseMode:   telebot.ModeHTML,
			ReplyMarkup: keyboard,
		},
	)
	return err
}

func (t Telegram) editTripCallback(c telebot.Context, tripID int64) error {
	text, err := t.DB.BuildTripMessage(tripID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to build message"})
	}

	cars, err := t.DB.GetCarsByTrip(tripID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to get cars"})
	}

	keyboard := buildKeyboard(cars, tripID)

	return c.Edit(text, &telebot.SendOptions{
		ParseMode:   telebot.ModeHTML,
		ReplyMarkup: keyboard,
	})
}

func buildKeyboard(cars []models.CarButton, tripID int64) *telebot.ReplyMarkup {
	var keyboard [][]telebot.InlineButton

	for _, car := range cars {
		keyboard = append(keyboard, []telebot.InlineButton{
			{Text: fmt.Sprintf("Join %s", car.Name), Data: fmt.Sprintf("join_%d", car.ID)},
		})
	}

	keyboard = append(keyboard, []telebot.InlineButton{
		{Text: "Add 🚙", Data: fmt.Sprintf("add_car_%d", tripID)},
	})

	return &telebot.ReplyMarkup{InlineKeyboard: keyboard}
}
