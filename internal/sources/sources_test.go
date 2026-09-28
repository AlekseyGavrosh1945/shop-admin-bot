package sources

import (
	"strings"
	"testing"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/poller"
	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
)

func TestFormatOrder(t *testing.T) {
	got := FormatOrder(storage.OrderEvent{
		OrderID: 12345, Status: "paid", Method: "ideal", Price: 1234.6, WhenText: "25.09 14:05",
	})
	for _, want := range []string{"#12345", "1 235 €", "paid", "ideal", "25.09 14:05"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatOrder missing %q: %s", want, got)
		}
	}
}

func TestFormatChatMessage(t *testing.T) {
	got := FormatChatMessage(storage.ChatMessageEvent{
		ChatID: 7, Username: "Jan", Email: "jan@example.com", Cause: 5, Text: "Is dit op voorraad?",
	})
	for _, want := range []string{"Jan", "#7", "вопрос о наличии", "Is dit op voorraad?"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatChatMessage missing %q: %s", want, got)
		}
	}

	// No username: fall back to email, then to a generic author.
	got = FormatChatMessage(storage.ChatMessageEvent{ChatID: 8, Email: "x@y.z"})
	if !strings.Contains(got, "x@y.z") {
		t.Errorf("email fallback missing: %s", got)
	}
	got = FormatChatMessage(storage.ChatMessageEvent{ChatID: 9})
	if !strings.Contains(got, "Клиент") {
		t.Errorf("generic author missing: %s", got)
	}
}

func TestFormatTicketEscapesDescription(t *testing.T) {
	got := FormatTicket(storage.TicketEvent{
		ID: 3, OrderID: "OR-1", Status: 2, Description: "<b>inject</b>",
	})
	if strings.Contains(got, "<b>inject</b>") {
		t.Errorf("description was not escaped: %s", got)
	}
	if !strings.Contains(got, "&lt;b&gt;inject&lt;/b&gt;") || !strings.Contains(got, "открыт") {
		t.Errorf("unexpected ticket rendering: %s", got)
	}
}

func TestFormatReviewAndRegistration(t *testing.T) {
	got := FormatReview(storage.ReviewEvent{Author: "Pieter", Rating: 4, Text: "Goed", Published: false})
	if !strings.Contains(got, "Pieter") || !strings.Contains(got, "4/5") || !strings.Contains(got, "ждёт модерации") {
		t.Errorf("FormatReview = %s", got)
	}

	got = FormatRegistration(storage.RegistrationEvent{ID: 5, Email: "a@b.c"})
	if !strings.Contains(got, "a@b.c") {
		t.Errorf("FormatRegistration = %s", got)
	}
}

func TestBatchText(t *testing.T) {
	events := []poller.Event{{ID: 1, Text: "line1"}, {ID: 2, Text: "line2"}}
	got := BatchText("orders", events)
	if !strings.Contains(got, "Новые заказы") || !strings.Contains(got, "line1") || !strings.Contains(got, "line2") {
		t.Errorf("BatchText = %s", got)
	}
}

func TestCauseAndStatusFallbacks(t *testing.T) {
	if got := causeName(99); got != "вопрос #99" {
		t.Errorf("causeName(99) = %q", got)
	}
	if got := ticketStatus(42); got != "статус 42" {
		t.Errorf("ticketStatus(42) = %q", got)
	}
}
