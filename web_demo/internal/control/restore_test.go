package control

import (
	"context"
	"errors"
	"testing"
)

func TestRestoreRequiresAdminCompletedRevocationAndFreshVersion(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if err := s.Bootstrap(ctx, roster()); err != nil {
		t.Fatal(err)
	}
	as, _ := s.Accounts(ctx)
	p, _ := s.Policy(ctx)
	if err := s.SyncResult(ctx, p.Revision, true); err != nil {
		t.Fatal(err)
	}
	admin, user := as[0], as[4]
	if err := s.Disable(ctx, admin.ID, user.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(ctx, user.ID, user.ID, 2); !errors.Is(err, ErrForbidden) {
		t.Fatal("non-admin restored account", err)
	}
	if err := s.Restore(ctx, admin.ID, user.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("unfinished revocation restored", err)
	}
	if err := s.RevokeComplete(ctx, user.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(ctx, admin.ID, user.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale version restored", err)
	}
	if err := s.Restore(ctx, admin.ID, user.ID, 2); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Account(ctx, user.ID)
	if got.Status != "pending" || got.EdgeReady || got.AuthVersion != 3 || got.Role != user.Role {
		t.Fatal("restore bypassed pending policy", got)
	}
	if _, err := s.Authenticate(ctx, user.Email, "subject"); !errors.Is(err, ErrForbidden) {
		t.Fatal("authenticated before reconciliation", err)
	}
	if err := s.Restore(ctx, admin.ID, user.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("restore replay accepted", err)
	}
	p, _ = s.Policy(ctx)
	if p.Revision != 3 {
		t.Fatal("unexpected policy revision", p.Revision)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT count(*) FROM audit WHERE target_id=? AND action='restore'`, user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing restore audit", count, err)
	}
}
