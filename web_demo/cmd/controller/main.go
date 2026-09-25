// controller has no browser listener. The operator initializes/migrates state;
// only the gateway's kernel UID may invoke its typed Unix control API.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/lifecycle"
)

type config struct {
	Database             string                        `json:"database"`
	Socket               string                        `json:"socket"`
	GatewaySocket        string                        `json:"gateway_socket"`
	RuntimeSocket        string                        `json:"runtime_socket"`
	GatewayUID           uint32                        `json:"gateway_uid"`
	BrokerUID            uint32                        `json:"broker_uid"`
	DeploymentID         string                        `json:"deployment_id"`
	ActivationGeneration int64                         `json:"activation_generation"`
	ImageID              string                        `json:"image_id"`
	Slots                []lifecycle.Slot              `json:"slots"`
	Workspaces           []lifecycle.Workspace         `json:"workspaces"`
	BrokerSocket         string                        `json:"broker_socket"`
	CollectorSocket      string                        `json:"collector_socket"`
	PipelineSocket       string                        `json:"pipeline_socket"`
	ReleaseRoot          string                        `json:"release_root"`
	ProfilePath          string                        `json:"profile_path"`
	Endpoints            map[string]lifecycle.Endpoint `json:"endpoints"`
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "controller:", e)
		os.Exit(1)
	}
}
func run() error {
	action := flag.String("action", "serve", "initialize, migrate, serve, activate, stop-intent, status, assign, restore-account or upgrade-image")
	file := flag.String("config", "", "operator-owned configuration")
	jobID := flag.String("job", "", "exact lifecycle job for operator review/abort")
	confirmSHA := flag.String("confirm-sha256", "", "reviewed exact job request hash")
	slotID := flag.String("slot", "", "fixed slot ID for prepare-reclaim")
	reclaimID := flag.String("reclaim", "", "exact reclaim intent for review/completion")
	workspaceID := flag.String("workspace", "", "exact configured workspace for an offline operator action")
	expectedGeneration := flag.Int64("expected-generation", 0, "reviewed current generation")
	expectedImage := flag.String("expected-image", "", "reviewed current image for upgrade-image")
	flag.Parse()
	if *file == "" || flag.NArg() != 0 {
		return errors.New("config required")
	}
	fi, e := os.Lstat(*file)
	if e != nil {
		return e
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0022 != 0 {
		return errors.New("unsafe configuration permissions")
	}
	f, e := os.Open(*file)
	if e != nil {
		return e
	}
	defer f.Close()
	var cfg config
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if e = dec.Decode(&cfg); e != nil {
		return e
	}
	if cfg.GatewayUID != 2102 || cfg.BrokerUID != 2104 || cfg.RuntimeSocket != "/run/user/2101/feam-docker.sock" {
		return errors.New("dedicated runtime and gateway UID required")
	}
	if *action != "status" && *action != "recovery-review" && *action != "retention-status" && *action != "reclaim-review" && *action != "activate" && *action != "stop-intent" {
		lock, err := lifecycle.ProcessLock(cfg.Database)
		if err != nil {
			return err
		}
		defer lock.Close()
	}
	// Initialization refuses to adopt private files, active runtimes, or partial
	// prior state. Ordinary converge always opens the existing database instead.
	if *action == "initialize" {
		for _, slot := range cfg.Slots {
			if e = lifecycle.VerifySlot(slot); e != nil {
				return e
			}
			entries, err := os.ReadDir(slot.Path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.Name() != "lost+found" {
					return errors.New("initialization refuses nonempty workspace slot")
				}
			}
		}
	}
	var d *sql.DB
	switch *action {
	case "initialize":
		d, e = db.Initialize(cfg.Database, lifecycle.Migrations)
	case "migrate":
		d, e = db.Migrate(cfg.Database, lifecycle.Migrations)
	default:
		d, e = db.Open(cfg.Database, lifecycle.Migrations)
	}
	if e != nil {
		return e
	}
	defer d.Close()
	store := lifecycle.New(d)
	switch *action {
	case "initialize":
		var ids []string
		for _, slot := range cfg.Slots {
			if e = lifecycle.VerifySlot(slot); e != nil {
				return e
			}
			ids = append(ids, slot.ID)
		}
		if e = store.InitializeDeployment(cfg.DeploymentID, cfg.ActivationGeneration, ids); e != nil {
			return e
		}
		for _, w := range cfg.Workspaces {
			if e = store.Assign(w); e != nil {
				return e
			}
		}
		return nil
	case "migrate":
		return nil
	case "assign":
		for _, w := range cfg.Workspaces {
			if w.ID == *workspaceID && w.DeploymentID == cfg.DeploymentID && w.ImageID == cfg.ImageID {
				if _, ok := cfg.Endpoints[w.ID]; !ok {
					return errors.New("workspace endpoint missing")
				}
				return store.AssignStopped(w)
			}
		}
		return errors.New("exact configured workspace required")
	case "activate":
		return store.Desired(true)
	case "stop-intent":
		return store.Desired(false)
	case "status":
		status, e := store.LifecycleStatus(context.Background())
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(status)
	case "recovery-review":
		review, err := store.ReviewRecovery(*jobID)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(review)
	case "retention-status":
		notices, err := store.RetentionNotices()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(notices)
	case "retention-warn":
		notices, err := store.WarnRetention()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(notices)
	case "reclaim-review":
		review, err := store.ReclaimReview(*reclaimID)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(review)
	case "serve", "abort-job", "prepare-reclaim", "complete-reclaim", "restore-account", "upgrade-image":
	default:
		return errors.New("unknown action")
	}
	slots := map[string]lifecycle.Slot{}
	for _, s := range cfg.Slots {
		if _, ok := slots[s.ID]; ok {
			return errors.New("duplicate slot")
		}
		slots[s.ID] = s
	}
	if !filepath.IsAbs(cfg.ProfilePath) || cfg.ReleaseRoot != "/home/feam-service-data/datasets/pipeline/releases" || cfg.BrokerSocket == "" || cfg.CollectorSocket == "" || cfg.PipelineSocket == "" || len(cfg.Endpoints) == 0 {
		return errors.New("complete hosted dependencies required")
	}
	profile, e := os.ReadFile(cfg.ProfilePath)
	if e != nil {
		return e
	}
	authority := lifecycle.HTTPAuthority{Client: ipc.Client(cfg.GatewaySocket), URL: "http://gateway"}
	provision := &lifecycle.Provisioner{Authority: authority, BrokerSocket: cfg.BrokerSocket, CollectorSocket: cfg.CollectorSocket, PipelineSocket: cfg.PipelineSocket, ReleaseRoot: cfg.ReleaseRoot, Profile: profile, Endpoints: cfg.Endpoints}
	runtimeClient := ipc.Client(cfg.RuntimeSocket)
	runtimeClient.Timeout = 30 * time.Second
	c := &lifecycle.Controller{Store: store, Authority: authority, Capabilities: provision, Runtime: &lifecycle.Docker{Client: runtimeClient, Deployment: cfg.DeploymentID, Slots: slots, Image: cfg.ImageID, Provision: provision, Store: store}}
	operatorCtx, cancelOperator := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelOperator()
	switch *action {
	case "restore-account", "upgrade-image":
		if _, ok := cfg.Endpoints[*workspaceID]; !ok {
			return errors.New("exact configured workspace required")
		}
		w, err := store.Workspace(*workspaceID)
		if err != nil {
			return err
		}
		// Inspect the recorded old container against its own immutable image.
		c.Runtime = &lifecycle.Docker{Client: runtimeClient, Deployment: cfg.DeploymentID, Slots: slots, Image: w.ImageID, Provision: provision, Store: store}
		if *action == "restore-account" {
			return c.RestoreAccount(operatorCtx, *workspaceID, *expectedGeneration)
		}
		return c.UpgradeImage(operatorCtx, *workspaceID, *expectedGeneration, *expectedImage, cfg.ImageID)
	case "abort-job":
		return c.AbortJob(operatorCtx, *jobID, *confirmSHA)
	case "prepare-reclaim":
		slot, ok := slots[*slotID]
		if !ok {
			return errors.New("unknown fixed slot")
		}
		if e = lifecycle.VerifySlot(slot); e != nil {
			return e
		}
		review, err := c.PrepareReclaim(operatorCtx, slot)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(review)
	case "complete-reclaim":
		return c.CompleteReclaim(*reclaimID)
	}
	l, e := ipc.Listen(cfg.Socket)
	if e != nil {
		return e
	}
	defer l.Close()
	handler := c.Handler()
	gate := ipc.RequireUID([]uint32{cfg.GatewayUID}, handler)
	operations := ipc.RequireUID([]uint32{cfg.BrokerUID}, handler)
	server := ipc.Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/workspaces/operation" {
			operations.ServeHTTP(w, r)
			return
		}
		gate.ServeHTTP(w, r)
	}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go c.Run(ctx)
	go func() { <-ctx.Done(); _ = server.Close() }()
	if err := server.Serve(l); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
