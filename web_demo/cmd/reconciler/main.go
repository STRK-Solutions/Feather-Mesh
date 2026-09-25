package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/reconcile"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	config := flag.String("config", "", "private JSON configuration path")
	once := flag.Bool("once", false, "one exact policy reconciliation")
	flag.Parse()
	f, e := os.Open(*config)
	if e != nil {
		log.Fatal("private configuration unavailable")
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		log.Fatal("configuration must be owner-only")
	}
	var c struct {
		ControlSocket string            `json:"control_socket"`
		AccountID     string            `json:"account_id"`
		UserGroup     string            `json:"user_group"`
		AdminGroup    string            `json:"admin_group"`
		GroupNames    map[string]string `json:"group_names"`
		TokenFile     string            `json:"token_file"`
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil {
		log.Fatal("invalid private configuration")
	}
	fi, e = os.Lstat(c.TokenFile)
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		log.Fatal("token must be an owner-only regular file")
	}
	token, e := os.ReadFile(c.TokenFile)
	if e != nil {
		log.Fatal("membership credential unavailable")
	}
	r := reconcile.Reconciler{Source: reconcile.RemoteSource{Socket: c.ControlSocket}, Provider: &reconcile.Cloudflare{AccountID: c.AccountID, Token: strings.TrimSpace(string(token)), GroupNames: c.GroupNames}, UserGroup: c.UserGroup, AdminGroup: c.AdminGroup}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	for {
		if e = r.Once(ctx); e != nil {
			log.Print("exact membership reconciliation pending")
			if *once {
				os.Exit(1)
			}
		}
		if *once {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}
