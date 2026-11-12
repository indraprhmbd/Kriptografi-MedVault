package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChallengeLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	expires := time.Now().UTC().Add(60 * time.Second).Format(time.RFC3339)

	if err := store.Challenges.Create(ctx, "abc123", "dr_andi", "LOGIN", expires); err != nil {
		t.Fatalf("Create: %v", err)
	}

	challenge, err := store.Challenges.Consume(ctx, "abc123")
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if challenge.Username != "dr_andi" {
		t.Fatalf("username mismatch: %q", challenge.Username)
	}
	if challenge.Purpose != "LOGIN" {
		t.Fatalf("purpose mismatch: %q", challenge.Purpose)
	}

	// Consuming twice must fail: the challenge is single use.
	if _, err := store.Challenges.Consume(ctx, "abc123"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on the second consume, got %v", err)
	}
}

func TestChallengeRejectsExpired(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	expired := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := store.Challenges.Create(ctx, "deadbeef", "dr_andi", "LOGIN", expired); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.Challenges.Consume(ctx, "deadbeef"); err == nil {
		t.Fatal("an expired challenge must not be accepted")
	}
}

func TestChallengeCleanupRemovesOnlyExpired(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	if err := store.Challenges.Create(ctx, "old", "dr_andi", "LOGIN", now.Add(-time.Minute).Format(time.RFC3339)); err != nil {
		t.Fatalf("create old: %v", err)
	}
	if err := store.Challenges.Create(ctx, "new", "dr_andi", "LOGIN", now.Add(time.Minute).Format(time.RFC3339)); err != nil {
		t.Fatalf("create new: %v", err)
	}

	removed, err := store.Challenges.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removal, got %d", removed)
	}

	if _, err := store.Challenges.Consume(ctx, "new"); err != nil {
		t.Fatalf("the live challenge must survive cleanup: %v", err)
	}
}

func TestWebSessionLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	expires := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	if err := store.WebSessions.Put(ctx, "session-token", "registration", "dr_andi", `{"challenge":"abc"}`, expires); err != nil {
		t.Fatalf("Put: %v", err)
	}

	session, err := store.WebSessions.Take(ctx, "session-token")
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	if session.Kind != "registration" {
		t.Fatalf("kind mismatch: %q", session.Kind)
	}
	if session.SessionJSON != `{"challenge":"abc"}` {
		t.Fatalf("session json mismatch: %q", session.SessionJSON)
	}

	// The ceremony data is single use.
	if _, err := store.WebSessions.Take(ctx, "session-token"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWebSessionRejectsExpired(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	expired := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	if err := store.WebSessions.Put(ctx, "t", "login", "dr_andi", `{"challenge":"abc"}`, expired); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := store.WebSessions.Take(ctx, "t"); err == nil {
		t.Fatal("an expired ceremony session must not be usable")
	}
}

func TestAppSessionLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	expires := now.Add(8 * time.Hour).Format(time.RFC3339)

	if err := store.AppSessions.Create(ctx, "sha-token", "dr_andi", "DONGLE-2026-dr_andi", now.Format(time.RFC3339), expires); err != nil {
		t.Fatalf("Create: %v", err)
	}

	session, err := store.AppSessions.Get(ctx, "sha-token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if session.Username != "dr_andi" {
		t.Fatalf("username mismatch: %q", session.Username)
	}
	if session.DongleKeyID != "DONGLE-2026-dr_andi" {
		t.Fatalf("dongle key id mismatch: %q", session.DongleKeyID)
	}

	if err := store.AppSessions.Touch(ctx, "sha-token", now.Add(time.Minute).Format(time.RFC3339)); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	if err := store.AppSessions.Delete(ctx, "sha-token"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.AppSessions.Get(ctx, "sha-token"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestAppSessionRejectsExpired(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	if err := store.AppSessions.Create(ctx, "old-token", "dr_andi", "DONGLE", now.Add(-9*time.Hour).Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.AppSessions.Get(ctx, "old-token"); err == nil {
		t.Fatal("an expired dashboard session must not be accepted")
	}
}

func TestAppSessionCleanup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	store.AppSessions.Create(ctx, "old", "dr_andi", "D", now.Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339))
	store.AppSessions.Create(ctx, "live", "dr_andi", "D", now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))

	removed, err := store.AppSessions.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removal, got %d", removed)
	}
	if _, err := store.AppSessions.Get(ctx, "live"); err != nil {
		t.Fatalf("live session must survive: %v", err)
	}
}

func TestWebAuthnCredentials(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	user := seedUser(t, store, "dr_andi")

	if err := store.WebAuthnCredentials.Add(ctx, user.ID, `{"id":"cred-1"}`); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.WebAuthnCredentials.Add(ctx, user.ID, `{"id":"cred-2"}`); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	list, err := store.WebAuthnCredentials.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(list))
	}

	if err := store.WebAuthnCredentials.DeleteAll(ctx, user.ID); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	list, err = store.WebAuthnCredentials.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no credentials, got %d", len(list))
	}
}

func TestWebAuthnCredentialRequiresExistingUser(t *testing.T) {
	store := newTestStore(t)

	if err := store.WebAuthnCredentials.Add(context.Background(), 4242, `{"id":"x"}`); err == nil {
		t.Fatal("the foreign key must reject a credential for a missing user")
	}
}

func TestStoreRejectsNilDependencies(t *testing.T) {
	if _, err := NewStore(nil, nil); err == nil {
		t.Fatal("expected an error for nil dependencies")
	}

	store := newTestStore(t)
	if errors.Is(nil, ErrNotFound) {
		t.Fatal("sanity check failed")
	}
	_ = store
}
