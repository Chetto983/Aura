package garageadmin

import (
	"context"
	"net/http"
	"testing"
)

// key_restore_test.go pins the two calls that put back a key Garage lost while Postgres kept its
// credential row. Measured 2026-10-19 on a throwaway Garage v2.4.1: GetKeyInfo?id= answers 404
// NoSuchAccessKey for an unknown key, ImportKey answers 200 the first time and 409
// KeyAlreadyExists after, and the imported key signs S3 requests with the secret it was given.

func TestKeyExistsByID(t *testing.T) {
	c, got := newMock(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = w.Write([]byte(`{"accessKeyId":"GK1","name":"aura-x"}`))
	})
	exists, err := c.KeyExists(context.Background(), "GK1")
	if err != nil || !exists {
		t.Fatalf("KeyExists(200) = %t, %v; want true, nil", exists, err)
	}
	if got.Method != http.MethodGet || got.Path != "/v2/GetKeyInfo" || got.Query != "id=GK1" {
		t.Fatalf("KeyExists sent %s %s?%s, want GET /v2/GetKeyInfo?id=GK1", got.Method, got.Path, got.Query)
	}
	if got.Auth != "Bearer "+testToken {
		t.Fatalf("KeyExists auth = %q", got.Auth)
	}
}

func TestKeyExistsReportsAnUnknownKey(t *testing.T) {
	c, _ := newMock(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NoSuchAccessKey"}`))
	})
	exists, err := c.KeyExists(context.Background(), "GK1")
	if err != nil || exists {
		t.Fatalf("KeyExists(404) = %t, %v; want false, nil", exists, err)
	}
}

// An admin API that could not answer is not a key that is gone: importing on that answer would
// try to create a key that may be there.
func TestKeyExistsFailsOnAnyOtherStatus(t *testing.T) {
	c, _ := newMock(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if exists, err := c.KeyExists(context.Background(), "GK1"); err == nil {
		t.Fatalf("KeyExists(500) = %t, nil; want an error", exists)
	}
}

func TestImportKeySendsTheStoredCredentials(t *testing.T) {
	c, got := newMock(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = w.Write([]byte(`{"accessKeyId":"GK1"}`))
	})
	if err := c.ImportKey(context.Background(), "GK1", "secret", "aura-x"); err != nil {
		t.Fatalf("ImportKey: %v", err)
	}
	if got.Method != http.MethodPost || got.Path != "/v2/ImportKey" {
		t.Fatalf("ImportKey sent %s %s, want POST /v2/ImportKey", got.Method, got.Path)
	}
	want := map[string]any{"accessKeyId": "GK1", "secretAccessKey": "secret", "name": "aura-x"}
	for k, v := range want {
		if got.Body[k] != v {
			t.Fatalf("ImportKey body[%q] = %v, want %v (body %v)", k, got.Body[k], v, got.Body)
		}
	}
}

func TestImportKeyFailsWhenGarageRefuses(t *testing.T) {
	c, _ := newMock(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"KeyAlreadyExists"}`))
	})
	if err := c.ImportKey(context.Background(), "GK1", "secret", "aura-x"); err == nil {
		t.Fatal("ImportKey(409) = nil, want an error")
	}
}
