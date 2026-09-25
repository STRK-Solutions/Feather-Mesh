// capture-fixture produces only a clearly labeled synthetic development export.
// It cannot contact a provider, submit real events or change research policy.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/collector"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func main() {
	outPath := flag.String("output", "", "new synthetic reviewed JSON export")
	flag.Parse()
	if *outPath == "" {
		log.Fatal("output required")
	}
	tmp, e := os.MkdirTemp("", "feam-synthetic-capture-")
	if e != nil {
		log.Fatal(e)
	}
	defer os.RemoveAll(tmp)
	d, e := db.Initialize(filepath.Join(tmp, "events.db"), collector.Migrations)
	if e != nil {
		log.Fatal(e)
	}
	defer d.Close()
	store := collector.New(d, 1<<20)
	archiveDir := filepath.Join(tmp, "archive")
	if e = os.Mkdir(archiveDir, 0700); e != nil {
		log.Fatal(e)
	}
	archive := collector.DirectoryArchive{Root: archiveDir}
	id := func(suffix string) string { return "00000000-0000-4000-8000-0000000000" + suffix }
	participant := id("01")
	receipt := []byte(`{"synthetic":true,"manifest_revision":"fixture-r1","commit":"confirmed"}`)
	ev := collector.Event{Protocol: collector.Protocol, EventID: id("02"), StreamID: id("03"), Sequence: 1, DeploymentID: id("04"), WorkspaceID: id("05"), Generation: 1, ParticipantID: participant, ConversationID: id("06"), RequestID: id("07"), OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Kind: "outcome", Trust: "independently_verified", SoftwareSHA256: strings.Repeat("a", 64), ProfileSHA256: strings.Repeat("b", 64), DatasetSHA256: strings.Repeat("c", 64), Synthetic: true, Payload: collector.Payload{Outcome: "committed", ReceiptSHA256: hash(receipt), TaskTemplate: "synthetic-resolve", DatasetFamily: "synthetic-climate"}}
	ev.Sanitize(false)
	ctx := context.Background()
	var reviews []collector.Review
	for i := 0; i < 2; i++ {
		if i == 1 {
			ev.EventID = id("08")
			ev.Sequence = 2
			ev.Kind = "review"
			ev.Trust = "client_reported"
			ev.Payload.Outcome = ""
			ev.Payload.ReceiptSHA256 = ""
			ev.Payload.Decision = "denied"
			ev.Sanitize(false)
		}
		if _, e = store.Append(ctx, ev); e != nil {
			log.Fatal(e)
		}
		b, _ := json.Marshal(ev)
		reviews = append(reviews, collector.Review{EventID: ev.EventID, EventSHA256: hash(b)})
	}
	export, e := store.Export(ctx, archive, "automated-synthetic-fixture", participant, "held_out", reviews)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(export, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	f, e := os.OpenFile(*outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		log.Fatal(e)
	}
	if _, e = f.Write(append(b, '\n')); e != nil {
		log.Fatal(e)
	}
	if e = f.Sync(); e != nil {
		log.Fatal(e)
	}
	if e = f.Close(); e != nil {
		log.Fatal(e)
	}
}
