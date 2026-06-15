package main

import (
	"car_organizer_bot/src/database"
	"car_organizer_bot/src/telegram"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Default().Printf("warn loading .env file: %v", err)
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("the TOKEN is not set in .env file")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./archive"
	}

	db := database.NewDatabase(dbPath)
	defer db.Close()

	db.CreateTables()

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     token,
		ParseMode: telebot.ModeHTML,
		Poller: &telebot.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: []string{"message", "callback_query"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	tg := telegram.Telegram{
		Bot: bot,
		DB:  db,
	}

	tg.SetupHandlers()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Default().Println("bot started")
		bot.Start()
	}()

	<-signalChan
	log.Default().Println("shutdown signal received.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		bot.Stop()
		log.Default().Println("bot stopped")
	}()

	<-shutdownCtx.Done()
	log.Default().Println("shutdown complete.")
}
