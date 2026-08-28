package auth

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		Username:      "admin",
		Password:      "a durable password",
		SessionSecret: "0123456789abcdef0123456789abcdef",
		APIToken:      "abcdef0123456789abcdef0123456789",
	}
}

func TestLoginSessionAndLogout(t *testing.T) {
	manager, err := NewManager(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	info, signed, expiry, err := manager.Login("127.0.0.1", "admin", "a durable password")
	if err != nil {
		t.Fatal(err)
	}
	if info.CSRF == "" || expiry.Sub(time.Now()) < 11*time.Hour {
		t.Fatal("expected a durable session and csrf token")
	}
	request, _ := http.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(manager.Cookie(signed, expiry))
	fromRequest, ok := manager.SessionFromRequest(request)
	if !ok || fromRequest.Username != "admin" || !manager.CheckCSRF(fromRequest, info.CSRF) {
		t.Fatal("expected signed session cookie to authenticate")
	}
	manager.Logout(fromRequest)
	if _, ok := manager.SessionFromRequest(request); ok {
		t.Fatal("logout should invalidate session")
	}
}

func TestBearerToken(t *testing.T) {
	manager, err := NewManager(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !manager.IsBearerToken("Bearer " + testConfig().APIToken) {
		t.Fatal("expected valid token")
	}
	if manager.IsBearerToken("Bearer wrong") {
		t.Fatal("wrong token must fail")
	}
}

func TestLoginAttemptTrackingIsBounded(t *testing.T) {
	manager, err := NewManager(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxTrackedLoginIPs; i++ {
		if _, _, _, err := manager.Login("ip-"+strconv.Itoa(i), "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d = %v", i, err)
		}
	}
	if _, _, _, err := manager.Login("overflow", "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected oldest-attempt eviction, got %v", err)
	}
	if len(manager.attempts) != maxTrackedLoginIPs {
		t.Fatalf("attempt tracking grew to %d entries", len(manager.attempts))
	}
}

func TestArchiveTicketIsSessionBoundAndOneUse(t *testing.T) {
	manager, err := NewManager(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := manager.IssueArchiveTicket("session-a", []ArchiveItem{{Path: "a.txt", Version: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.ConsumeArchiveTicket("session-b", ticket); ok {
		t.Fatal("ticket must be bound to session")
	}
	items, ok := manager.ConsumeArchiveTicket("session-a", ticket)
	if !ok || len(items) != 1 {
		t.Fatal("ticket should work once for issuing session")
	}
	if _, ok := manager.ConsumeArchiveTicket("session-a", ticket); ok {
		t.Fatal("ticket must be one-use")
	}
}
