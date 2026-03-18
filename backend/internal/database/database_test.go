//go:build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConnect_Success(t *testing.T) {
	// Arrange
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Act
	pool, err := Connect(ctx, databaseURL)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Errorf("ping failed after connect: %v", err)
	}
}

func TestConnect_UnreachableHost(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	pool, err := Connect(ctx, "postgres://invalid:invalid@localhost:9999/nonexistent?sslmode=disable")

	// Assert
	if err == nil {
		pool.Close()
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestConnect_MalformedURL(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	pool, err := Connect(ctx, "not-a-valid-url")

	// Assert
	if err == nil {
		pool.Close()
		t.Fatal("expected error for malformed URL, got nil")
	}
}
