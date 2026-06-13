package model

import (
	"testing"
	"time"
)

func TestNewUser_WithPassword(t *testing.T) {
	u := NewUser("1", "john_doe", "john@example.com", "password123")

	if u.ID != "1" {
		t.Errorf("expected ID to be '1', got %q", u.ID)
	}
	if u.Username != "john_doe" {
		t.Errorf("expected Username to be 'john_doe', got %q", u.Username)
	}
	if u.Email != "john@example.com" {
		t.Errorf("expected Email to be 'john@example.com', got %q", u.Email)
	}
	if !u.IsActive {
		t.Error("expected IsActive to be true when password is provided")
	}
	if u.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be initialized")
	}
}

func TestNewUser_WithoutPassword(t *testing.T) {
	u := NewUser("2", "jane_doe", "jane@example.com", "")

	if u.IsActive {
		t.Error("expected IsActive to be false when password is empty")
	}
}

func TestUser_Deactivate(t *testing.T) {
	u := NewUser("1", "john_doe", "john@example.com", "password123")
	initialTime := u.UpdatedAt

	// 時間経過をシミュレートするため少し待つ
	time.Sleep(10 * time.Millisecond)

	u.Deactivate()

	if u.IsActive {
		t.Error("expected IsActive to be false after deactivation")
	}
	if !u.UpdatedAt.After(initialTime) {
		t.Errorf("expected UpdatedAt to be updated to a later time, initial: %v, current: %v", initialTime, u.UpdatedAt)
	}
}
