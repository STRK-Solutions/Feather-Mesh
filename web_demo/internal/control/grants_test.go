package control

import (
	"context"
	"strings"
	"testing"
)

func TestGrantTransactionVersionsAdmissionAndDurableAck(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if e := s.Bootstrap(ctx, roster()); e != nil {
		t.Fatal(e)
	}
	as, _ := s.Accounts(ctx)
	w := Workspace{ID: ID(), OwnerID: as[4].ID, Hostname: "u-" + as[4].ID + ".example.invalid", Socket: "/run/private.sock", Ready: true, Generation: 1, GrantVersion: 1, DeploymentID: ID(), ActivationGeneration: 1}
	if e := s.Assign(ctx, w); e != nil {
		t.Fatal(e)
	}
	p, _ := s.Policy(ctx)
	if e := s.SyncResult(ctx, p.Revision, true); e != nil {
		t.Fatal(e)
	}
	digest := strings.Repeat("a", 64)
	if e := s.SetGrant(ctx, as[4].ID, as[4].ID, "climate", digest, 1); e != ErrForbidden {
		t.Fatalf("user grant: %v", e)
	}
	if e := s.SetGrant(ctx, as[0].ID, as[4].ID, "climate", digest, 1); e != nil {
		t.Fatal(e)
	}
	assign, e := s.Assignments(ctx, w.ID)
	if e != nil || assign.Workspace.Ready || assign.Workspace.GrantVersion != 2 || assign.Account.AuthVersion != 2 || len(assign.Grants) != 1 || assign.Grants[0].Digest != digest {
		t.Fatalf("incomplete atomic grant: %+v %v", assign, e)
	}
	if _, e = s.Authorize(ctx, Check{AccountID: as[4].ID, AuthVersion: 2, WorkspaceID: w.ID, GrantVersion: 2}); e == nil {
		t.Fatal("old mounted workspace admitted")
	}
	if e = s.Snapshot(ctx, w); e != ErrConflict {
		t.Fatalf("stale controller snapshot: %v", e)
	}
	if e = s.SetGrant(ctx, as[0].ID, as[4].ID, "climate", strings.Repeat("b", 64), 1); e != ErrConflict {
		t.Fatalf("stale grant accepted: %v", e)
	}
	if e = s.SetGrant(ctx, as[0].ID, as[4].ID, "climate", "", 2); e != nil {
		t.Fatal(e)
	}
	pins, e := s.ReleasePins(ctx)
	if e != nil || len(pins) != 1 || pins[0].Digest != digest {
		t.Fatal("pending mount revocation lost retention pin")
	}
	if e = s.GrantUpdateComplete(ctx, w.ID, 2); e != ErrConflict {
		t.Fatalf("stale ack: %v", e)
	}
	pending, e := s.PendingGrantUpdates(ctx)
	if e != nil || len(pending) != 1 || pending[0].GrantVersion != 3 {
		t.Fatalf("lost pending policy: %+v %v", pending, e)
	}
	assign, e = s.Assignments(ctx, w.ID)
	if e != nil || len(assign.Grants) != 0 {
		t.Fatal("revoked release still assigned")
	}
	if e = s.GrantUpdateComplete(ctx, w.ID, 3); e != nil {
		t.Fatal(e)
	}
	pending, e = s.PendingGrantUpdates(ctx)
	if e != nil || len(pending) != 0 {
		t.Fatal("ack not persisted")
	}
	pins, e = s.ReleasePins(ctx)
	if e != nil || len(pins) != 0 {
		t.Fatal("completed revoked grant remained pinned")
	}
	if e = s.Disable(ctx, as[0].ID, as[4].ID, 3); e != nil {
		t.Fatal(e)
	}
	if e = s.SetGrant(ctx, as[0].ID, as[4].ID, "climate", digest, 3); e != ErrForbidden {
		t.Fatal("disabled participant granted")
	}
}

func TestBundleWithdrawalRevocationIsDurableAndIdempotent(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	s.Bootstrap(ctx, roster())
	as, _ := s.Accounts(ctx)
	w := Workspace{ID: ID(), OwnerID: as[4].ID, Hostname: "u-" + as[4].ID + ".example.invalid", Socket: "/run/private.sock", Ready: true, Generation: 1, GrantVersion: 1, DeploymentID: ID(), ActivationGeneration: 1}
	if e := s.Assign(ctx, w); e != nil {
		t.Fatal(e)
	}
	p, _ := s.Policy(ctx)
	s.SyncResult(ctx, p.Revision, true)
	if e := s.SetGrant(ctx, as[0].ID, as[4].ID, "climate", strings.Repeat("a", 64), 1); e != nil {
		t.Fatal(e)
	}
	if e := s.RevokeBundleGrants(ctx, as[4].ID, "climate"); e != ErrForbidden {
		t.Fatal("participant revoked bundle")
	}
	if e := s.RevokeBundleGrants(ctx, as[0].ID, "climate"); e != nil {
		t.Fatal(e)
	}
	if e := s.RevokeBundleGrants(ctx, as[0].ID, "climate"); e != nil {
		t.Fatal("lost acknowledgment not idempotent", e)
	}
	assign, e := s.Assignments(ctx, w.ID)
	if e != nil || len(assign.Grants) != 0 || assign.Workspace.GrantVersion != 3 || assign.Account.AuthVersion != 3 || assign.Workspace.Ready {
		t.Fatal("withdrawal repeated or incomplete", e)
	}
	pins, e := s.ReleasePins(ctx)
	if e != nil || len(pins) != 1 {
		t.Fatal("withdrawal lost pending mount pin")
	}
	if e = s.GrantUpdateComplete(ctx, w.ID, 3); e != nil {
		t.Fatal(e)
	}
	pins, e = s.ReleasePins(ctx)
	if e != nil || len(pins) != 0 {
		t.Fatal("acknowledged withdrawal stayed pinned")
	}
}
