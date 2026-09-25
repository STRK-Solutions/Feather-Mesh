// project-budget is an operator-only CLI. Never install its key material on a
// disposable host; the broker receives only the signed run and public key.
package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/broker"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	action := flag.String("action", "status", "keygen|initialize|status|allocate|unknown|reconcile|ceiling")
	path := flag.String("ledger", "", "operator encrypted ledger path")
	keyPath := flag.String("encryption-key", "", "private 32-byte key file")
	signPath := flag.String("signing-key", "", "private raw 64-byte Ed25519 key file")
	publicPath := flag.String("public-key", "", "public raw 32-byte Ed25519 verification key file")
	actor := flag.String("actor", "", "authenticated operator audit identity")
	project := flag.String("project", "", "project UUID for initialization")
	amount := flag.Int64("amount", 0, "USD microdollars (ceiling or verified final charge)")
	prior := flag.Int64("prior-charges", -1, "explicit prior attributable charges for initialization")
	document := flag.String("allocation", "", "unsigned allocation JSON file")
	id := flag.String("allocation-id", "", "allocation UUID")
	evidence := flag.String("evidence-sha256", "", "verified shutdown/provider receipt hash")
	flag.Parse()
	if *action == "keygen" {
		return budget.InitializeKeys(*keyPath, *signPath, *publicPath)
	}
	key, err := budget.ReadPrivateKey(*keyPath, 32)
	if err != nil {
		return err
	}
	s := budget.ProjectStore{Path: *path, Key: key}
	switch *action {
	case "initialize":
		return s.Initialize(*project, *actor, *amount, *prior)
	case "status":
		p, err := s.Read()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(p)
	case "allocate":
		var a budget.Allocation
		b, err := os.ReadFile(*document)
		if err != nil {
			return err
		}
		if err = broker.StrictDecode(b, &a); err != nil {
			return err
		}
		key, err := budget.ReadPrivateKey(*signPath, ed25519.PrivateKeySize)
		if err != nil {
			return err
		}
		signed, err := s.Allocate(*actor, a, ed25519.PrivateKey(key))
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(signed)
	case "unknown":
		return s.MarkUnknown(*actor, *id)
	case "reconcile":
		return s.Reconcile(*actor, *id, *evidence, *amount)
	case "ceiling":
		return s.SetCeiling(*actor, *amount)
	default:
		return budget.ErrInvalid
	}
}
