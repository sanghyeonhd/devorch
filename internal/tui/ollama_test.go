package tui

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOllamaClientChat(t *testing.T) {
	client := NewOllamaClient()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	msgs := []Message{{Role: "user", Content: "Say hello"}}
	resp, err := client.Chat(ctx, "tinyllama:latest", msgs)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	fmt.Printf("Response: %s\n", resp)
	if resp == "" {
		t.Error("Expected non-empty response")
	}
}
