package pipeline

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueuedJobDurableAuditAndNoReplay(t *testing.T) {
	m := managerFixture(t)
	j := fixtureJob()
	ctx := context.Background()
	s, e := m.Enqueue(ctx, j, j.Hash(), "00000000-0000-4000-8000-000000000099")
	if e != nil || s.State != "queued" {
		t.Fatalf("queue: %+v %v", s, e)
	}
	var audits int
	m.DB.QueryRow(`SELECT count(*) FROM pipeline_audit WHERE action='enqueue' AND job_id=?`, j.ID).Scan(&audits)
	if audits != 1 {
		t.Fatal("queue lost authorizing audit")
	}
	if _, e = m.Enqueue(ctx, j, j.Hash(), "00000000-0000-4000-8000-000000000099"); e != ErrReconcile {
		t.Fatal("queue replay accepted")
	}
	m.runNext(ctx)
	s, _ = m.Status(j.ID)
	if s.State != "unknown" {
		t.Fatal("worker failure not sticky")
	}
	m.runNext(ctx)
	s, _ = m.Status(j.ID)
	if s.State != "unknown" {
		t.Fatal("unknown worker automatically replayed")
	}
	jobs, e := m.Jobs()
	if e != nil || len(jobs) != 1 || jobs[0].ID != j.ID {
		t.Fatal("durable job listing failed")
	}
}

func TestAdminIPCRejectsActorHeadersAndClosedWithdrawal(t *testing.T) {
	m := managerFixture(t)
	r := httptest.NewRequest("POST", "http://pipeline/v1/jobs/enqueue", strings.NewReader(`{"actor":"00000000-0000-4000-8000-000000000099"}`))
	r.Header.Set("X-UID", "200")
	w := httptest.NewRecorder()
	m.ServiceHandler(100, 200).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("untrusted actor header granted mutation")
	}
	j := fixtureJob()
	j.Withdrawal = &Withdrawal{Targets: []WithdrawalTarget{{ID: "daily", Version: "v1"}}, Reason: "test"}
	b, _ := json.Marshal(j)
	if _, e := DecodeJob(b); e == nil {
		t.Fatal("withdrawal without parent accepted")
	}
	j.ParentDigest = strings.Repeat("a", 64)
	j.Withdrawal.Targets = append(j.Withdrawal.Targets, j.Withdrawal.Targets[0])
	b, _ = json.Marshal(j)
	if _, e := DecodeJob(b); e == nil {
		t.Fatal("duplicate withdrawal accepted")
	}
}
