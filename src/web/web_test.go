package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"car_organizer_bot/src/database"
	"car_organizer_bot/src/models"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
)

type mockSlackClient struct {
	PostMessageCalled   bool
	UpdateMessageCalled bool
	LastChannel         string
	LastTimestamp       string
	LastOptions         []slack.MsgOption
}

func (m *mockSlackClient) PostMessage(channel string, options ...slack.MsgOption) (string, string, error) {
	m.PostMessageCalled = true
	m.LastChannel = channel
	m.LastOptions = options
	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	return channel, ts, nil
}

func (m *mockSlackClient) UpdateMessage(channel, timestamp string, options ...slack.MsgOption) (string, string, string, error) {
	m.UpdateMessageCalled = true
	m.LastChannel = channel
	m.LastTimestamp = timestamp
	m.LastOptions = options
	return channel, timestamp, "", nil
}

func newTestServer(t *testing.T) (*WebServer, *mockSlackClient, *gin.Engine) {
	t.Helper()
	dir, err := os.MkdirTemp("", "car_organizer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	db := database.NewDatabase(dir)
	db.CreateTables()
	mock := &mockSlackClient{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return &WebServer{
		DB:          db,
		SlackClient: mock,
		Port:        "0",
	}, mock, r
}

func TestHTMLToMrkdwn(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>bold</b>", "*bold*"},
		{"<i>italic</i>", "_italic_"},
		{"<code>code</code>", "`code`"},
		{"<b>bold</b> and <i>italic</i>", "*bold* and _italic_"},
		{"plain text", "plain text"},
		{"nested <b><i>both</i></b>", "nested *_both_*"},
	}
	for _, tt := range tests {
		result := htmlToMrkdwn(tt.input)
		if result != tt.expected {
			t.Fatalf("htmlToMrkdwn(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestBuildTripBlocks(t *testing.T) {
	blocks := buildTripBlocks(1, "*Trip Test*\n\nCars here", nil)
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks (section + actions), got %d", len(blocks))
	}
	actionBlock, ok := blocks[1].(*slack.ActionBlock)
	if !ok {
		t.Fatalf("expected block[1] to be *ActionBlock, got %T", blocks[1])
	}
	if len(actionBlock.Elements.ElementSet) != 2 {
		t.Fatalf("expected 2 elements (Add + Leave) with nil cars, got %d", len(actionBlock.Elements.ElementSet))
	}
}

func TestBuildTripBlocksWithCars(t *testing.T) {
	cars := []models.CarButton{
		{ID: 10, Name: "Alice"},
		{ID: 11, Name: "Bob"},
	}
	blocks := buildTripBlocks(1, "*Trip*", cars)
	actionBlock := blocks[1].(*slack.ActionBlock)
	if len(actionBlock.Elements.ElementSet) != 4 {
		t.Fatalf("expected 4 elements (Join x2 + Add + Leave), got %d", len(actionBlock.Elements.ElementSet))
	}
}

func TestCreateTripCommand(t *testing.T) {
	ws, mock, r := newTestServer(t)

	r.POST("/", ws.HandleSlashCommand)

	body := url.Values{
		"command":    {"/trip"},
		"text":       {"Beach Trip"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !mock.PostMessageCalled {
		t.Fatal("expected PostMessage to be called")
	}

	trip, err := ws.DB.SelectTripByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if trip.Name != "Beach Trip" {
		t.Fatalf("expected 'Beach Trip', got '%s'", trip.Name)
	}
}

func TestCreateTripNoName(t *testing.T) {
	ws, _, r := newTestServer(t)
	r.POST("/", ws.HandleSlashCommand)

	body := url.Values{
		"command":    {"/trip"},
		"text":       {""},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	bodyBytes, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(bodyBytes), "Please provide a trip name") {
		t.Fatalf("expected name prompt, got: %s", string(bodyBytes))
	}
}

func TestAddCarAndJoinFlow(t *testing.T) {
	ws, mock, r := newTestServer(t)
	r.POST("/webhook", ws.HandleInteractive)

	tripID, _ := ws.DB.InsertTrip("C123", "Trip")
	ws.DB.AddCar(tripID, "U1", "Alice")
	ws.DB.UpdateTripMessageID(tripID, fmt.Sprintf("%d", time.Now().UnixNano()))

	payload := slack.InteractionCallback{
		Channel: slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C123"}}},
		User:    slack.User{ID: "U2", Name: "Bob"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{Value: fmt.Sprintf("join_%d", getFirstCarID(t, ws.DB, tripID))},
			},
		},
	}
	payloadJSON, _ := json.Marshal(payload)
	body := url.Values{"payload": {string(payloadJSON)}}
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !mock.UpdateMessageCalled {
		t.Fatal("expected UpdateMessage to be called after join")
	}
}

func TestSeatsCommand(t *testing.T) {
	ws, _, r := newTestServer(t)
	r.POST("/", ws.HandleSlashCommand)

	tripID, _ := ws.DB.InsertTrip("C123", "Trip")
	ws.DB.AddCar(tripID, "U1", "Alice")

	body := url.Values{
		"command":    {"/seats"},
		"text":       {"3"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNameCommand(t *testing.T) {
	ws, mock, r := newTestServer(t)
	r.POST("/", ws.HandleSlashCommand)

	tripID, _ := ws.DB.InsertTrip("C123", "Trip")
	ws.DB.AddCar(tripID, "U1", "Alice")
	ws.DB.UpdateTripMessageID(tripID, "ts1")

	body := url.Values{
		"command":    {"/name"},
		"text":       {"Alicia"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !mock.UpdateMessageCalled {
		t.Fatal("expected UpdateMessage to be called for trip message refresh")
	}
}

func TestLeaveTripInteractive(t *testing.T) {
	ws, _, r := newTestServer(t)
	r.POST("/webhook", ws.HandleInteractive)

	tripID, _ := ws.DB.InsertTrip("C123", "Trip")
	ws.DB.AddCar(tripID, "U1", "Alice")
	ws.DB.UpdateTripMessageID(tripID, "ts1")

	payload := slack.InteractionCallback{
		Channel: slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C123"}}},
		User:    slack.User{ID: "U1"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{Value: fmt.Sprintf("leave_%d", tripID)},
			},
		},
	}
	payloadJSON, _ := json.Marshal(payload)
	body := url.Values{"payload": {string(payloadJSON)}}
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cars, _ := ws.DB.GetCarsByTrip(tripID)
	if len(cars) != 0 {
		t.Fatalf("expected 0 cars after leave, got %d", len(cars))
	}
}

func TestVerifyRequestSkipsWhenNoSecret(t *testing.T) {
	ws, _, r := newTestServer(t)
	r.POST("/", ws.verifyRequest, ws.HandleSlashCommand)

	body := url.Values{
		"command":    {"/trip"},
		"text":       {"Trip"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when no signing secret, got %d", w.Code)
	}
}

func TestVerifyRequestRejectsInvalid(t *testing.T) {
	ws, _, r := newTestServer(t)
	ws.SigningSecret = "my-secret"
	r.POST("/", ws.verifyRequest, ws.HandleSlashCommand)

	body := url.Values{
		"command":    {"/trip"},
		"text":       {"Trip"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}
	req, _ := http.NewRequest("POST", "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-Slack-Signature", "v0=fake")
	w := performRequest(r, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid signature, got %d", w.Code)
	}
}

func TestVerifyRequestAcceptsValid(t *testing.T) {
	ws, _, r := newTestServer(t)
	secret := "my-secret"
	ws.SigningSecret = secret
	r.POST("/", ws.verifyRequest, ws.HandleSlashCommand)

	now := fmt.Sprintf("%d", time.Now().Unix())
	bodyStr := url.Values{
		"command":    {"/trip"},
		"text":       {"Trip"},
		"channel_id": {"C123"},
		"user_id":    {"U1"},
	}.Encode()

	base := fmt.Sprintf("v0:%s:%s", now, bodyStr)
	mac := hmacSHA256([]byte(secret), []byte(base))
	expectedSig := "v0=" + mac

	req, _ := http.NewRequest("POST", "/", strings.NewReader(bodyStr))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", now)
	req.Header.Set("X-Slack-Signature", expectedSig)
	w := performRequest(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", w.Code)
	}
}

func getFirstCarID(t *testing.T, db *database.Database, tripID int64) int64 {
	t.Helper()
	cars, err := db.GetCarsByTrip(tripID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cars) == 0 {
		t.Fatal("no cars found")
	}
	return cars[0].ID
}

func performRequest(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func hmacSHA256(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
