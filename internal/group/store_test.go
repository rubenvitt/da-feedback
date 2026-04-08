package group_test

import (
	"context"
	"testing"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/group"
)

func setupTestDB(t *testing.T) *group.Store {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return group.NewStore(db)
}

func TestCreateAndGetGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, err := store.Create(ctx, "I&K", "iuk")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.Name != "I&K" || g.Slug != "iuk" || len(g.Secret) != 5 {
		t.Fatalf("unexpected group: %+v", g)
	}

	got, err := store.GetByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "I&K" {
		t.Fatalf("expected I&K, got %s", got.Name)
	}
}

func TestGetBySlugAndSecret(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created, err := store.Create(ctx, "Sanitätsdienst", "san")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetBySlugAndSecret(ctx, created.Slug, created.Secret)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %d, got %d", created.ID, got.ID)
	}

	_, err = store.GetBySlugAndSecret(ctx, "san", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestListGroups(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, "I&K", "iuk")
	store.Create(ctx, "Sanitätsdienst", "san")

	groups, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2, got %d", len(groups))
	}
}

func TestUpdateGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	hours := 24
	g.Name = "IuK"
	g.CloseAfterHours = &hours
	err := store.Update(ctx, g)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetByID(ctx, g.ID)
	if got.Name != "IuK" || *got.CloseAfterHours != 24 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestDeleteGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	err := store.Delete(ctx, g.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.GetByID(ctx, g.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRegenerateSecret(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	oldSecret := g.Secret

	newSecret, err := store.RegenerateSecret(ctx, g.ID)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if newSecret == oldSecret {
		t.Fatal("secret should have changed")
	}
	if len(newSecret) != 5 {
		t.Fatalf("expected 5 chars, got %d", len(newSecret))
	}
}
