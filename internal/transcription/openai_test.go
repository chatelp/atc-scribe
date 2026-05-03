package transcription

import (
	"strings"
	"testing"
)

func TestParseTranscriptionSessionResponseUsesTopLevelID(t *testing.T) {
	body := []byte(`{
		"object": "realtime.transcription_session",
		"id": "sess_test123",
		"client_secret": {
			"value": "ek_test_secret",
			"expires_at": 1777749999
		}
	}`)

	sessionID, clientSecret, err := parseTranscriptionSessionResponse(body)
	if err != nil {
		t.Fatalf("parseTranscriptionSessionResponse returned error: %v", err)
	}

	if sessionID != "sess_test123" {
		t.Fatalf("sessionID = %q, want %q", sessionID, "sess_test123")
	}
	if clientSecret != "ek_test_secret" {
		t.Fatalf("clientSecret = %q, want %q", clientSecret, "ek_test_secret")
	}
}

func TestParseTranscriptionSessionResponseRejectsMissingID(t *testing.T) {
	body := []byte(`{
		"object": "realtime.transcription_session",
		"client_secret": {"value": "ek_test_secret"}
	}`)

	_, _, err := parseTranscriptionSessionResponse(body)
	if err == nil {
		t.Fatal("expected error for missing session id")
	}
	if !strings.Contains(err.Error(), "session id") {
		t.Fatalf("error = %q, want mention of session id", err.Error())
	}
}

func TestRealtimeTranscriptionWebSocketURLUsesIntentTranscription(t *testing.T) {
	got := realtimeTranscriptionWebSocketURL()
	want := "wss://api.openai.com/v1/realtime?intent=transcription"
	if got != want {
		t.Fatalf("realtimeTranscriptionWebSocketURL() = %q, want %q", got, want)
	}
}

func TestReconnectableWebSocketErrorsIncludesOpenAIKeepaliveTimeout(t *testing.T) {
	err := "websocket: close 1011 (internal server error): keepalive ping timeout"
	if !isReconnectableWebSocketError(err) {
		t.Fatalf("expected %q to be reconnectable", err)
	}
}
