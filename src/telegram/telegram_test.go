package telegram

import (
	"car_organizer_bot/src/models"
	"testing"

	"gopkg.in/telebot.v3"
)

func makeUser(username, firstName, lastName string) *telebot.User {
	return &telebot.User{
		ID:        12345,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
	}
}

func TestDefineUsernameWithUsername(t *testing.T) {
	user := makeUser("john_doe", "John", "Doe")
	name := DefineUsername(user)
	if name != "john_doe" {
		t.Fatalf("expected 'john_doe', got '%s'", name)
	}
}

func TestDefineUsernameWithFirstLast(t *testing.T) {
	user := makeUser("", "John", "Doe")
	name := DefineUsername(user)
	if name != "J.Doe" {
		t.Fatalf("expected 'J.Doe', got '%s'", name)
	}
}

func TestDefineUsernameWithFirstNameOnly(t *testing.T) {
	user := makeUser("", "John", "")
	name := DefineUsername(user)
	if name != "John" {
		t.Fatalf("expected 'John', got '%s'", name)
	}
}

func TestDefineUsernameFallbackToID(t *testing.T) {
	user := &telebot.User{ID: 99999}
	name := DefineUsername(user)
	if name != "ID:99999" {
		t.Fatalf("expected 'ID:99999', got '%s'", name)
	}
}

func TestBuildKeyboardEmptyCars(t *testing.T) {
	kb := buildKeyboard(nil, 1)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows (Add + Leave), got %d", len(kb.InlineKeyboard))
	}
}

func TestBuildKeyboardWithCars(t *testing.T) {
	cars := []models.CarButton{
		{ID: 10, Name: "Alice"},
		{ID: 11, Name: "Bob"},
	}
	kb := buildKeyboard(cars, 1)
	expectedRows := len(cars) + 2
	if len(kb.InlineKeyboard) != expectedRows {
		t.Fatalf("expected %d rows, got %d", expectedRows, len(kb.InlineKeyboard))
	}

	joinBtn := kb.InlineKeyboard[0][0]
	expected := "Join Alice"
	if joinBtn.Text != expected {
		t.Fatalf("expected '%s', got '%s'", expected, joinBtn.Text)
	}
	if joinBtn.Data != "join_10" {
		t.Fatalf("expected 'join_10', got '%s'", joinBtn.Data)
	}

	addBtn := kb.InlineKeyboard[len(cars)][0]
	if addBtn.Text != "Add 🚙" {
		t.Fatalf("expected 'Add 🚙', got '%s'", addBtn.Text)
	}
	if addBtn.Data != "add_car_1" {
		t.Fatalf("expected 'add_car_1', got '%s'", addBtn.Data)
	}

	leaveBtn := kb.InlineKeyboard[len(cars)+1][0]
	if leaveBtn.Text != "Leave trip" {
		t.Fatalf("expected 'Leave trip', got '%s'", leaveBtn.Text)
	}
	if leaveBtn.Data != "leave_1" {
		t.Fatalf("expected 'leave_1', got '%s'", leaveBtn.Data)
	}
}
