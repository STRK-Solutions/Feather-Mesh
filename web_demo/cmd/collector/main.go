package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/collector"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	DB                string            `json:"db"`
	CapabilitiesDB    string            `json:"capabilities_db"`
	ControlSocket     string            `json:"control_socket"`
	GatewaySocket     string            `json:"gateway_socket"`
	WorkspaceSockets  map[string]string `json:"workspace_sockets"`
	BrokerUID         uint32            `json:"broker_uid"`
	ControllerUID     uint32            `json:"controller_uid"`
	VerifierUID       uint32            `json:"verifier_uid"`
	ReviewerUIDs      []uint32          `json:"reviewer_uids"`
	Reviewer          string            `json:"reviewer"`
	PolicyFile        string            `json:"policy_file"`
	ArchiveMode       string            `json:"archive_mode"`
	ArchiveDirectory  string            `json:"archive_directory"`
	R2CredentialsFile string            `json:"r2_credentials_file"`
	R2AccountID       string            `json:"r2_account_id"`
	R2Bucket          string            `json:"r2_bucket"`
	LocalMaxBytes     int64             `json:"local_max_bytes"`
	ArchiveMaxBytes   int64             `json:"archive_max_bytes"`
}

const ubuntuArchiveDirectory = "/home/feam-service-data/traces/collector/archive"

func selectedArchive(c config) (collector.Archive, error) {
	switch c.ArchiveMode {
	case "local":
		if c.ArchiveDirectory != ubuntuArchiveDirectory || c.R2CredentialsFile != "" || c.R2AccountID != "" || c.R2Bucket != "" {
			return nil, errors.New("local archive requires fixed private Ubuntu path and no R2 inputs")
		}
		return collector.OpenDirectoryArchive(c.ArchiveDirectory, c.DB)
	case "r2":
		if c.ArchiveDirectory != "" {
			return nil, errors.New("R2 archive cannot use a local directory")
		}
		var credential struct {
			AccessKeyID     string `json:"access_key_id"`
			SecretAccessKey string `json:"secret_access_key"`
		}
		if readPrivate(c.R2CredentialsFile, &credential) != nil || credential.AccessKeyID == "" || credential.SecretAccessKey == "" {
			return nil, errors.New("private archive credentials unavailable")
		}
		return collector.R2{AccountID: c.R2AccountID, Bucket: c.R2Bucket, AccessKeyID: credential.AccessKeyID, SecretAccessKey: credential.SecretAccessKey}, nil
	default:
		return nil, errors.New("explicit supported archive mode required")
	}
}

func readPrivate(path string, v any) error {
	raw, e := readPrivateData(path)
	if e != nil {
		return e
	}
	return decodePrivate(raw, v)
}
func readPrivateData(path string) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 1<<20 {
		return nil, errors.New("private bounded regular configuration required")
	}
	identity, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || identity.Nlink != 1 {
		return nil, errors.New("configuration must not have hardlink aliases")
	}
	raw, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return nil, errors.New("private configuration exceeds bound")
	}
	return raw, nil
}
func decodePrivate(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing configuration")
	}
	return nil
}
func main() {
	mode := flag.String("mode", "serve", "initialize, migrate, recover-index, flush, teardown-check, archive-verify, initialize-archive-ledger, export-archive-ledger, import-archive-ledger or serve")
	path := flag.String("config", "", "private configuration path")
	includeInventory := flag.Bool("include-inventory", false, "include bounded private archive object inventory in archive-verify receipt")
	ledgerPath := flag.String("ledger", "", "private prior archive ledger for explicit import into an untouched database")
	ledgerSHA := flag.String("confirm-ledger-sha256", "", "SHA256 of exact reviewed private ledger file")
	flag.Parse()
	if *includeInventory && *mode != "archive-verify" {
		log.Fatal("inventory is only available for archive-verify")
	}
	if (*ledgerPath != "" || *ledgerSHA != "") && *mode != "import-archive-ledger" {
		log.Fatal("ledger input is only available for explicit import")
	}
	var c config
	if readPrivate(*path, &c) != nil {
		log.Fatal("invalid private collector configuration")
	}
	if c.LocalMaxBytes <= 0 || c.LocalMaxBytes > 20<<30 || c.ArchiveMaxBytes <= 0 || c.ArchiveMaxBytes > 1<<30 {
		log.Fatal("reviewed local/archive capacity exceeded")
	}
	open := db.Open
	switch *mode {
	case "initialize":
		open = db.Initialize
	case "migrate":
		open = db.Migrate
	case "recover-index", "flush", "teardown-check", "archive-verify", "initialize-archive-ledger", "export-archive-ledger", "import-archive-ledger", "serve":
	default:
		log.Fatal("unknown explicit operation")
	}
	d, e := open(c.DB, collector.Migrations)
	if e != nil {
		log.Fatal(e)
	}
	defer d.Close()
	store := collector.New(d, c.LocalMaxBytes)
	store.MaxArchiveBytes = c.ArchiveMaxBytes
	if _, e = d.Exec(`PRAGMA secure_delete=ON`); e != nil {
		log.Fatal("secure deletion unavailable")
	}
	if *mode == "initialize" {
		caps, e := capability.Initialize(c.CapabilitiesDB)
		if e != nil {
			log.Fatal(e)
		}
		caps.DB.Close()
		return
	}
	if *mode == "migrate" {
		return
	}
	if *mode == "recover-index" {
		if e = store.Recover(context.Background()); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *mode == "teardown-check" {
		if e = store.TeardownReady(context.Background()); e != nil {
			log.Fatal(e)
		}
		return
	}
	archive, e := selectedArchive(c)
	if e != nil {
		log.Fatal(e)
	}
	if *mode == "initialize-archive-ledger" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		hash, err := store.InitializeArchiveBaseline(ctx, archive)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"protocol": "feam.archive-import.v1", "status": "initialized", "ledger_sha256": hash}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *mode == "export-archive-ledger" || *mode == "import-archive-ledger" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if *mode == "export-archive-ledger" {
			ledger, err := store.ExportArchiveLedger(ctx, archive)
			if err != nil {
				log.Fatal(err)
			}
			if err = json.NewEncoder(os.Stdout).Encode(ledger); err != nil {
				log.Fatal(err)
			}
		} else {
			var ledger collector.ArchiveLedger
			raw, err := readPrivateData(*ledgerPath)
			if err != nil || len(raw) > collector.MaxLedgerBytes {
				log.Fatal("invalid archive ledger bound")
			}
			h := sha256.Sum256(raw)
			if hex.EncodeToString(h[:]) != *ledgerSHA {
				log.Fatal("exact archive ledger approval hash mismatch")
			}
			if err = decodePrivate(raw, &ledger); err != nil {
				log.Fatal("invalid private archive ledger")
			}
			if err = store.ImportArchiveLedger(ctx, archive, ledger); err != nil {
				log.Fatal(err)
			}
			if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"protocol": "feam.archive-import.v1", "status": "imported", "ledger_sha256": ledger.SHA256}); err != nil {
				log.Fatal(err)
			}
		}
		return
	}
	if *mode == "archive-verify" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		receipt, err := store.VerifyArchiveWithInventory(ctx, archive, *includeInventory)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(receipt); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *mode == "flush" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if e = store.Flush(ctx, archive); e != nil {
			log.Fatal("archive verification failed; retain local state")
		}
		if e = store.ReconcileDeletions(ctx, archive); e != nil {
			log.Fatal("deletion reconciliation pending")
		}
		if e = store.TeardownReady(ctx); e != nil {
			log.Fatal(e)
		}
		return
	}
	if c.BrokerUID == 0 || c.ControllerUID == 0 || c.VerifierUID == 0 || len(c.ReviewerUIDs) == 0 || c.Reviewer == "" {
		log.Fatal("separate trusted service and research-reviewer UIDs required")
	}
	if e = store.ArchiveBaselineReady(context.Background()); e != nil {
		log.Fatal(e)
	}
	if c.BrokerUID == c.ControllerUID || c.BrokerUID == c.VerifierUID || c.ControllerUID == c.VerifierUID {
		log.Fatal("broker, controller and independent verifier must use distinct UIDs")
	}
	for _, reviewer := range c.ReviewerUIDs {
		if reviewer < 1000 || reviewer >= 2101 && reviewer <= 2107 || reviewer == 200999 || reviewer == c.BrokerUID || reviewer == c.ControllerUID || reviewer == c.VerifierUID {
			log.Fatal("research reviewers must be distinct from trusted service UIDs")
		}
	}
	var participants []collector.Participant
	if readPrivate(c.PolicyFile, &participants) != nil {
		log.Fatal("current administrative consent policy unavailable")
	}
	mapping := map[string]collector.Participant{}
	for _, p := range participants {
		if _, exists := mapping[p.AccountID]; exists {
			log.Fatal("duplicate participant policy")
		}
		mapping[p.AccountID] = p
	}
	caps, e := capability.Open(c.CapabilitiesDB)
	if e != nil {
		log.Fatal(e)
	}
	defer caps.DB.Close()
	s := &collector.Server{Store: store, Capabilities: caps, Authority: collector.ControlAuthority{Client: ipc.Client(c.GatewaySocket)}, Participants: mapping, Archive: archive, BrokerUID: c.BrokerUID, ControllerUID: c.ControllerUID, VerifierUID: c.VerifierUID, ReviewerUIDs: c.ReviewerUIDs, Reviewer: c.Reviewer}
	if e = store.Recover(context.Background()); e != nil {
		log.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errs := make(chan error, len(c.WorkspaceSockets)+1)
	var servers []*http.Server
	serve := func(path string, h http.Handler) {
		l, e := ipc.Listen(path)
		if e != nil {
			log.Fatal(e)
		}
		srv := ipc.Server(h)
		servers = append(servers, srv)
		go func() { errs <- srv.Serve(l) }()
	}
	serve(c.ControlSocket, s.Control())
	for workspace, socket := range c.WorkspaceSockets {
		serve(socket, s.Workspace(workspace))
	}
	go func() {
		tick := time.NewTicker(5 * time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				flushCtx, done := context.WithTimeout(ctx, 45*time.Second)
				if e := store.Flush(flushCtx, archive); e != nil {
					log.Print("research archive pending; teardown blocked")
				}
				if e := store.ReconcileDeletions(flushCtx, archive); e != nil {
					log.Print("research deletion reconciliation pending")
				}
				if e := store.Expire(flushCtx, archive); e != nil {
					log.Print("research retention expiry pending")
				}
				done()
			}
		}
	}()
	select {
	case <-ctx.Done():
	case <-errs:
		cancel()
	}
	shutdown, done := context.WithTimeout(context.Background(), 45*time.Second)
	defer done()
	for _, srv := range servers {
		_ = srv.Shutdown(shutdown)
	}
	if e = store.Flush(shutdown, archive); e != nil {
		log.Print("shutdown archive verification failed; retain local state")
		os.Exit(1)
	}
	if e = store.ReconcileDeletions(shutdown, archive); e != nil {
		log.Print("shutdown deletion reconciliation pending; retain local state")
		os.Exit(1)
	}
	if e = store.TeardownReady(shutdown); e != nil {
		log.Print("teardown blocked by pending research archive")
		os.Exit(1)
	}
}
