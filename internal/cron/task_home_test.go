package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// fakeTaskConversations answers whether a conversation still exists and opens new ones.
type fakeTaskConversations struct {
	exists    map[string]bool
	existsErr error
	createID  string
	checked   []string
	created   []createdConversation
}

type createdConversation struct {
	identityID string
	title      string
}

func (f *fakeTaskConversations) ConversationExists(_ context.Context, conversationID, _ string) (bool, error) {
	f.checked = append(f.checked, conversationID)
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.exists[conversationID], nil
}

func (f *fakeTaskConversations) CreateTaskConversation(_ context.Context, identityID, title string) (string, error) {
	f.created = append(f.created, createdConversation{identityID: identityID, title: title})
	return f.createID, nil
}

// fakeTaskOrigins records the conversation a task was moved into.
type fakeTaskOrigins struct {
	set map[string]string
}

func (f *fakeTaskOrigins) SetTaskOrigin(_ context.Context, taskID, conversationID string) error {
	if f.set == nil {
		f.set = map[string]string{}
	}
	f.set[taskID] = conversationID
	return nil
}

func homeDispatch(handler *fakeHandler, convs *fakeTaskConversations, origins *fakeTaskOrigins, recorder *fakeConversationRecorder) *Dispatch {
	return NewDispatch(map[TaskKind]Handler{KindAgentJob: handler}, DispatchDeps{
		Store:                &fakeCompleter{},
		ConversationRecorder: recorder,
		TaskConversations:    convs,
		TaskOrigins:          origins,
	})
}

const dailyGoal = `{"goal": "Cerca le notizie principali del giorno in italiano, riassumile in modo conciso e inviale all'utente su Telegram"}`

// TestATaskWhoseConversationWasDeletedAnswersInOneOfItsOwn pins the defect measured on the
// lab VM on 2026-10-14: the operator bulk-deleted their conversations, the daily goal's
// origin among them, and its next run lost every tool-ledger row to the conversation foreign
// key and its outcome to "conversation not found". A task outlives the conversation it was
// scheduled from: it moves into one of its own, keeps it, and answers there.
func TestATaskWhoseConversationWasDeletedAnswersInOneOfItsOwn(t *testing.T) {
	t.Parallel()

	handler := &fakeHandler{summary: "le notizie di oggi"}
	convs := &fakeTaskConversations{exists: map[string]bool{}, createID: "conv-own"}
	origins := &fakeTaskOrigins{}
	recorder := &fakeConversationRecorder{}
	d := homeDispatch(handler, convs, origins, recorder)

	task := Task{ID: "task-1", Kind: KindAgentJob, NotifyRoute: "telegram", IdentityID: "id-1",
		OriginConversationID: "conv-deleted", Payload: []byte(dailyGoal)}
	if err := d.Dispatch(context.Background(), task, &Claim{RunID: "run-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if len(convs.created) != 1 || convs.created[0].identityID != "id-1" {
		t.Fatalf("created %+v, want one conversation owned by the task's identity", convs.created)
	}
	if got := origins.set["task-1"]; got != "conv-own" {
		t.Fatalf("the task now answers in %q, want it to keep conv-own for its next runs", got)
	}
	if got := handler.gotJob.OriginConversationID; got != "conv-own" {
		t.Fatalf("the run keyed its tool ledger on %q, want conv-own", got)
	}
	if len(recorder.appended) != 1 || recorder.appended[0].conversationID != "conv-own" {
		t.Fatalf("the outcome went to %+v, want conv-own", recorder.appended)
	}
}

// TestATaskWhoseConversationExistsStaysThere keeps the 2026-08-26 rule: the outcome goes
// where the operator asked for it, as long as that conversation is there.
func TestATaskWhoseConversationExistsStaysThere(t *testing.T) {
	t.Parallel()

	handler := &fakeHandler{summary: "done"}
	convs := &fakeTaskConversations{exists: map[string]bool{"conv-1": true}}
	origins := &fakeTaskOrigins{}
	d := homeDispatch(handler, convs, origins, &fakeConversationRecorder{})

	task := Task{ID: "task-1", Kind: KindAgentJob, NotifyRoute: "stdout", IdentityID: "id-1",
		OriginConversationID: "conv-1", Payload: []byte(dailyGoal)}
	if err := d.Dispatch(context.Background(), task, &Claim{RunID: "run-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(convs.created) != 0 || len(origins.set) != 0 {
		t.Fatalf("a task whose conversation exists was moved: created %+v, set %+v", convs.created, origins.set)
	}
	if got := handler.gotJob.OriginConversationID; got != "conv-1" {
		t.Fatalf("the run answered in %q, want conv-1", got)
	}
}

// TestATaskScheduledOutsideAConversationIsNotGivenOne leaves a CLI or system task alone:
// it never had a conversation to lose.
func TestATaskScheduledOutsideAConversationIsNotGivenOne(t *testing.T) {
	t.Parallel()

	convs := &fakeTaskConversations{}
	d := homeDispatch(&fakeHandler{}, convs, &fakeTaskOrigins{}, &fakeConversationRecorder{})

	task := Task{ID: "task-1", Kind: KindAgentJob, NotifyRoute: "stdout", IdentityID: "id-1", Payload: []byte(dailyGoal)}
	if err := d.Dispatch(context.Background(), task, &Claim{RunID: "run-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(convs.checked) != 0 || len(convs.created) != 0 {
		t.Fatalf("a task with no conversation was checked %v and given %+v", convs.checked, convs.created)
	}
}

// TestAnUnansweredExistenceCheckMovesNothing: a database that cannot say whether the
// conversation exists is no evidence that it was deleted.
func TestAnUnansweredExistenceCheckMovesNothing(t *testing.T) {
	t.Parallel()

	handler := &fakeHandler{}
	convs := &fakeTaskConversations{existsErr: errors.New("pool exhausted")}
	origins := &fakeTaskOrigins{}
	d := homeDispatch(handler, convs, origins, &fakeConversationRecorder{})

	task := Task{ID: "task-1", Kind: KindAgentJob, NotifyRoute: "stdout", IdentityID: "id-1",
		OriginConversationID: "conv-1", Payload: []byte(dailyGoal)}
	if err := d.Dispatch(context.Background(), task, &Claim{RunID: "run-1"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(convs.created) != 0 || len(origins.set) != 0 || handler.gotJob.OriginConversationID != "conv-1" {
		t.Fatalf("an unanswered check moved the task: created %+v, set %+v, ran in %q",
			convs.created, origins.set, handler.gotJob.OriginConversationID)
	}
}

// TestATaskConversationIsNamedAfterWhatTheTaskDoes: the sidebar is where the operator finds
// it, so the title is the task's own goal or text, short enough for one line.
func TestATaskConversationIsNamedAfterWhatTheTaskDoes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		task    Task
		want    string
		maxRune int
	}{
		{name: "goal", task: Task{Kind: KindAgentJob, Payload: []byte(dailyGoal)}, want: "⏰ Cerca le notizie principali del giorno", maxRune: taskTitleRunes},
		{name: "reminder text", task: Task{Kind: KindReminder, Payload: []byte(`{"text": "bevi acqua"}`)}, want: "⏰ bevi acqua"},
		{name: "no text", task: Task{Kind: KindReminder, Payload: []byte(`{}`)}, want: "⏰ reminder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := taskConversationTitle(tc.task)
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("title %q, want it to start with %q", got, tc.want)
			}
			if tc.maxRune > 0 && utf8.RuneCountInString(got) > tc.maxRune {
				t.Fatalf("title %q has %d runes, want at most %d", got, utf8.RuneCountInString(got), tc.maxRune)
			}
		})
	}
}
