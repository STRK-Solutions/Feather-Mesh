package control

import (
	"context"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	d, e := db.Initialize(filepath.Join(t.TempDir(), "control.db"), Migrations)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return &Store{DB: d}
}
func roster() []Enrollment {
	var r []Enrollment
	for i := 0; i < 9; i++ {
		role := "user"
		if i < 4 {
			role = "admin"
		}
		r = append(r, Enrollment{Email: fmt.Sprintf("u%d@example.invalid", i), Role: role})
	}
	return r
}
func TestBootstrapPendingRolesAndNeverResurrect(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if e := s.Bootstrap(ctx, roster()); e != nil {
		t.Fatal(e)
	}
	accounts, e := s.Accounts(ctx)
	if e != nil || len(accounts) != 9 {
		t.Fatal(e)
	}
	for _, a := range accounts {
		if a.Status != "pending" {
			t.Fatal("not pending")
		}
		if _, e = s.WorkspaceForOwner(ctx, a.ID); e == nil {
			t.Fatal("bootstrap allocated workspace")
		}
	}
	p, _ := s.Policy(ctx)
	if e = s.SyncResult(ctx, p.Revision, true); e != nil {
		t.Fatal(e)
	}
	admin, _ := s.Account(ctx, accounts[0].ID)
	user, _ := s.Account(ctx, accounts[4].ID)
	if admin.Status != "active" || user.Status != "pending" {
		t.Fatal("activation prerequisites ignored")
	}
	s.DB.Exec(`UPDATE accounts SET role='user' WHERE id=?`, accounts[1].ID)
	if e = s.Disable(ctx, admin.ID, user.ID, user.AuthVersion); e != nil {
		t.Fatal(e)
	}
	if e = s.Bootstrap(ctx, roster()); e != nil {
		t.Fatal(e)
	}
	user, _ = s.Account(ctx, user.ID)
	role, _ := s.Account(ctx, accounts[1].ID)
	if user.Status != "disabled" || role.Role != "user" {
		t.Fatal("bootstrap restored prior policy")
	}
	if _, e = s.DB.Exec(`UPDATE accounts SET id=? WHERE id=?`, ID(), admin.ID); e == nil {
		t.Fatal("mutable local ID")
	}
	pending, _ := s.PendingRevocations(ctx)
	if len(pending) != 1 {
		t.Fatal("lost durable revocation")
	}
}
func TestIdentityBindingOwnershipAndVersions(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	s.Bootstrap(ctx, roster())
	as, _ := s.Accounts(ctx)
	user := as[4]
	ws := Workspace{ID: ID(), OwnerID: user.ID, Hostname: "u-" + user.ID + ".example.invalid", Socket: "/run/feam/terminal.sock", Ready: true, Generation: 1, GrantVersion: 1, DeploymentID: ID(), ActivationGeneration: 1}
	if e := s.Assign(ctx, ws); e != nil {
		t.Fatal(e)
	}
	p, _ := s.Policy(ctx)
	if e := s.SyncResult(ctx, p.Revision, true); e != nil {
		t.Fatal(e)
	}
	a, e := s.Authenticate(ctx, user.Email, "verified-subject")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authenticate(ctx, user.Email, "different-subject"); e == nil {
		t.Fatal("subject substitution accepted")
	}
	if _, e = s.Authenticate(ctx, "unknown@example.invalid", "verified-subject"); e == nil {
		t.Fatal("unknown email admitted")
	}
	check := Check{AccountID: a.ID, AuthVersion: 1, WorkspaceID: ws.ID, WorkspaceGeneration: 1, GrantVersion: 1, DeploymentID: ws.DeploymentID, ActivationGeneration: 1}
	if _, e = s.Authorize(ctx, check); e != nil {
		t.Fatal(e)
	}
	check.AccountID = as[0].ID
	if _, e = s.Authorize(ctx, check); e == nil {
		t.Fatal("admin obtained private workspace")
	}
	check.AccountID = a.ID
	check.WorkspaceGeneration = 2
	if _, e = s.Authorize(ctx, check); e == nil {
		t.Fatal("stale generation accepted")
	}
	if e = s.Disable(ctx, as[0].ID, a.ID, 1); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authenticate(ctx, a.Email, "verified-subject"); e == nil {
		t.Fatal("disabled identity admitted")
	}
	if e = s.SyncResult(ctx, p.Revision, true); e == nil {
		t.Fatal("stale policy acknowledgment accepted")
	}
	if e = s.Assign(ctx, Workspace{ID: ID(), OwnerID: as[0].ID, Hostname: "u-" + as[0].ID + ".example.invalid", Socket: "/run/x", Generation: 1, GrantVersion: 1, DeploymentID: ID(), ActivationGeneration: 1}); e == nil {
		t.Fatal("admin workspace allocated")
	}
}
func TestEmailDoesNotCollapseAliases(t *testing.T) {
	for _, s := range []string{"a.b+x@example.invalid", "ab@example.invalid"} {
		if got, e := Email(s); e != nil || got != s {
			t.Fatal("altered identity")
		}
	}
	for _, s := range []string{" A@example.invalid", "A@example.invalid", "Name <a@example.invalid>"} {
		if _, e := Email(s); e == nil {
			t.Fatal("noncanonical identity")
		}
	}
}

func TestBootstrapAcceptsReviewedCohortWithinWorkspaceCapacity(t *testing.T) {
	for _, users := range []int{0, 1, 6, 10, 11} {
		t.Run(fmt.Sprint(users), func(t *testing.T) {
			s := fixture(t)
			ctx := context.Background()
			in := append([]Enrollment(nil), roster()[:4]...)
			for i := 0; i < users; i++ {
				in = append(in, Enrollment{Email: fmt.Sprintf("participant%d@example.invalid", i), Role: "user"})
			}
			err := s.Bootstrap(ctx, in)
			accounts, readErr := s.Accounts(ctx)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if users == 0 || users > 10 {
				if err == nil || len(accounts) != 0 {
					t.Fatal("out-of-capacity bootstrap mutated accounts")
				}
				return
			}
			if err != nil || len(accounts) != 4+users {
				t.Fatalf("reviewed cohort was not imported: %v", err)
			}
			for _, a := range accounts {
				if a.Status != "pending" || a.EdgeReady {
					t.Fatal("bootstrap activated a participant")
				}
			}
			// Converge with another valid roster must not enroll or remove anyone.
			if err = s.Bootstrap(ctx, roster()); err != nil {
				t.Fatal(err)
			}
			after, err := s.Accounts(ctx)
			if err != nil || len(after) != len(accounts) {
				t.Fatal("repeat bootstrap changed reviewed membership")
			}
		})
	}
}

func TestStagingBootstrapReconcilesOnlySelectedIdentities(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	selected := []Enrollment{{Email: "admin@example.invalid", Role: "admin"},
		{Email: "user@example.invalid", Role: "user"},
		{Email: "user+second@example.invalid", Role: "user"}}
	if err := s.Bootstrap(ctx, selected); err != nil {
		t.Fatal(err)
	}
	policy, err := s.Policy(ctx)
	if err != nil || len(policy.Admins) != 1 || policy.Admins[0] != selected[0].Email || len(policy.Users) != 2 {
		t.Fatal("staging policy includes an unselected identity", err)
	}
	if err = s.SyncResult(ctx, policy.Revision, true); err != nil {
		t.Fatal(err)
	}
	// Later full-roster converge cannot silently activate the remaining cohort.
	if err = s.Bootstrap(ctx, roster()); err != nil {
		t.Fatal(err)
	}
	accounts, err := s.Accounts(ctx)
	if err != nil || len(accounts) != 3 {
		t.Fatal("staging bootstrap broadened the cohort", err)
	}
	for _, a := range accounts {
		if a.Role == "admin" && a.Status != "active" || a.Role == "user" && a.Status != "pending" {
			t.Fatal("workspace or edge activation prerequisite bypassed")
		}
	}
	for _, admins := range []int{0, 5} {
		in := []Enrollment{{Email: "regular@example.invalid", Role: "user"}}
		for i := 0; i < admins; i++ {
			in = append(in, Enrollment{Email: fmt.Sprintf("admin%d@example.invalid", i), Role: "admin"})
		}
		if err = fixture(t).Bootstrap(ctx, in); err == nil {
			t.Fatal("invalid admin count accepted")
		}
	}
}
