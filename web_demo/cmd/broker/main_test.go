package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/broker"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func TestServiceShutdownCheckpointPreservesRunState(t *testing.T) {
	for _, paused := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "paused"}[paused], func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp", "feam-broker-service-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			id := "00000000-0000-4000-8000-000000000001"
			pub, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			a := budget.Allocation{Protocol: budget.Protocol, ID: id, ProjectID: id, RunID: id, DeploymentID: id,
				ActivationGeneration: 1, LedgerRevision: 1, Amount: 1000000, RequestLimit: 1000000, UserDailyLimit: 1000000,
				Model: budget.Model, Provider: budget.Provider, Profile: budget.Profile, InputPrice: 100000, OutputPrice: 500000,
				FeeBasisPoints: 10000, MaxOutputTokens: 64, NotBefore: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour)}
			doc, err := budget.Sign(a, key)
			if err != nil {
				t.Fatal(err)
			}
			run, err := budget.InitializeRun(filepath.Join(dir, "run.db"), doc, pub, id, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer run.Close()
			if paused {
				if err = run.Pause(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			s := broker.NewServer(run, nil, nil, broker.FakeProvider{})
			s.ReceiptFile = filepath.Join(dir, "receipt.json")
			c := config{ControlSocket: filepath.Join(dir, "control.sock"), ControllerUID: uint32(os.Getuid()), GatewayUID: uint32(os.Getuid() + 1)}
			c.Sockets = append(c.Sockets, struct {
				WorkspaceID string `json:"workspace_id"`
				Path        string `json:"path"`
			}{id, filepath.Join(dir, "model.sock")})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- serve(ctx, s, c) }()
			client := ipc.Client(c.ControlSocket)
			deadline := time.Now().Add(2 * time.Second)
			for {
				resp, err := client.Get("http://unix/v1/status")
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode != 200 {
						t.Fatal(resp.StatusCode)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal(err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("service shutdown did not finish")
			}
			b, err := os.ReadFile(s.ReceiptFile)
			if err != nil {
				t.Fatal(err)
			}
			var receipt budget.DrainReceipt
			if json.Unmarshal(b, &receipt) != nil || receipt.Usage.Paused != paused || receipt.AllocationSHA256 != doc.Digest() {
				t.Fatal("service shutdown changed admission state or failed receipt binding", receipt)
			}
		})
	}
}
