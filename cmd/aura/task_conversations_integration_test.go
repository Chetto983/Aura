//go:build db_integration

package main

import (
	"testing"

	"github.com/chetto1983/aura/internal/conversations"
	"github.com/chetto1983/aura/internal/identity"
	"github.com/chetto1983/aura/internal/identityctx"
	"github.com/chetto1983/aura/internal/runner"
	"github.com/google/uuid"
)

// TestTaskConversationsSeeADeletedConversationAndOpenAnOwnedOne drives the adapter the cron
// dispatch uses when a task's conversation was deleted (internal/cron/task_home.go): a
// conversation soft-deleted from the cockpit or removed outright counts as gone, and the
// replacement belongs to the task's identity and carries the task's title.
func TestTaskConversationsSeeADeletedConversationAndOpenAnOwnedOne(t *testing.T) {
	pool := mcpAuditMigratedPool(t)
	owner, convID := uuid.NewString(), uuid.NewString()
	ctx := identityctx.WithIdentityID(t.Context(), owner)
	if _, err := pool.Exec(ctx, "INSERT INTO aura.identities (id, name, kind) VALUES ($1,$2,'user')", owner, "task-home-"+owner); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(t.Context(), "DELETE FROM aura.identities WHERE id=$1", owner) })
	conv := conversations.New(pool, conversations.Config{RunDir: t.TempDir(), TurnCapBytes: 65536})
	if _, err := conv.Create(ctx, conversations.CreateParams{ID: convID, IdentityID: owner, Model: "test"}); err != nil {
		t.Fatal(err)
	}
	homes := conversationRecorder{store: conv, runner: runner.New(runner.Deps{Conv: conv, Identity: identity.New(pool)})}

	if exists, err := homes.ConversationExists(ctx, convID, owner); err != nil || !exists {
		t.Fatalf("a live conversation: exists=%v err=%v", exists, err)
	}
	if err := conv.UpdateStatus(ctx, convID, conversations.StatusDeleted); err != nil {
		t.Fatal(err)
	}
	if exists, err := homes.ConversationExists(ctx, convID, owner); err != nil || exists {
		t.Fatalf("a conversation deleted from the cockpit: exists=%v err=%v", exists, err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM aura.conversations WHERE id=$1", convID); err != nil {
		t.Fatal(err)
	}
	if exists, err := homes.ConversationExists(ctx, convID, owner); err != nil || exists {
		t.Fatalf("a conversation removed outright: exists=%v err=%v", exists, err)
	}

	id, err := homes.CreateTaskConversation(t.Context(), owner, "⏰ le notizie del giorno")
	if err != nil {
		t.Fatalf("CreateTaskConversation: %v", err)
	}
	got, err := conv.GetForIdentity(t.Context(), id, owner)
	if err != nil {
		t.Fatalf("the task's conversation is not the task identity's: %v", err)
	}
	if got.Title != "⏰ le notizie del giorno" || got.Status != conversations.StatusActive {
		t.Fatalf("the task's conversation is %q/%q, want the task's title and active", got.Title, got.Status)
	}
}
