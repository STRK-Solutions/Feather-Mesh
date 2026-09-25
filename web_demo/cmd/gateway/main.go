package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/auth"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/gateway"
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
	DB            string         `json:"db"`
	BrowserSocket string         `json:"browser_socket"`
	ControlSocket string         `json:"control_socket"`
	Issuer        string         `json:"issuer"`
	ReadUIDs      []uint32       `json:"read_uids"`
	ControllerUID uint32         `json:"controller_uid"`
	ReconcilerUID uint32         `json:"reconciler_uid"`
	PipelineUID   uint32         `json:"pipeline_uid"`
	Gateway       gateway.Config `json:"gateway"`
}

func privateJSON(path string, v any) error {
	fi, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		return errors.New("private owner-only regular file required")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}
func main() {
	mode := flag.String("mode", "serve", "initialize, migrate, bootstrap, invite, restore, assign or serve")
	path := flag.String("config", "", "private operator config")
	input := flag.String("input", "", "private roster or workspace assignment JSON")
	flag.Parse()
	var c config
	if e := privateJSON(*path, &c); e != nil {
		log.Fatal("invalid private gateway configuration")
	}
	open := db.Open
	switch *mode {
	case "initialize":
		open = db.Initialize
	case "migrate":
		open = db.Migrate
	case "bootstrap", "invite", "restore", "assign", "serve":
	default:
		log.Fatal("unknown explicit operation")
	}
	d, e := open(c.DB, control.Migrations)
	if e != nil {
		log.Fatal(e)
	}
	defer d.Close()
	s := &control.Store{DB: d}
	switch *mode {
	case "initialize", "migrate":
		return
	case "invite":
		var in struct {
			Actor      string             `json:"actor_id"`
			Enrollment control.Enrollment `json:"enrollment"`
		}
		if privateJSON(*input, &in) != nil {
			log.Fatal("invalid private enrollment")
		}
		account, err := s.Invite(context.Background(), in.Actor, in.Enrollment)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(account); err != nil {
			log.Fatal(err)
		}
		return
	case "restore":
		var in struct {
			Actor   string `json:"actor_id"`
			Target  string `json:"account_id"`
			Version int64  `json:"auth_version"`
		}
		if privateJSON(*input, &in) != nil {
			log.Fatal("invalid private restore request")
		}
		if err := s.Restore(context.Background(), in.Actor, in.Target, in.Version); err != nil {
			log.Fatal(err)
		}
		return
	case "bootstrap":
		var roster []control.Enrollment
		if privateJSON(*input, &roster) != nil {
			log.Fatal("invalid private roster")
		}
		if e = s.Bootstrap(context.Background(), roster); e != nil {
			log.Fatal(e)
		}
		return
	case "assign": // Socket location is operator configuration, never browser input.
		var in struct {
			ID                   string `json:"id"`
			OwnerID              string `json:"owner_id"`
			Hostname             string `json:"hostname"`
			Socket               string `json:"socket"`
			Ready                bool   `json:"ready"`
			Generation           int64  `json:"generation"`
			GrantVersion         int64  `json:"grant_version"`
			DeploymentID         string `json:"deployment_id"`
			ActivationGeneration int64  `json:"activation_generation"`
		}
		if privateJSON(*input, &in) != nil {
			log.Fatal("invalid private assignment")
		}
		if e = s.Assign(context.Background(), control.Workspace{ID: in.ID, OwnerID: in.OwnerID, Hostname: in.Hostname, Socket: in.Socket, Ready: in.Ready, Generation: in.Generation, GrantVersion: in.GrantVersion, DeploymentID: in.DeploymentID, ActivationGeneration: in.ActivationGeneration}); e != nil {
			log.Fatal(e)
		}
		return
	}
	if c.ControllerUID == 0 || c.ReconcilerUID == 0 || len(c.ReadUIDs) == 0 || c.ControlSocket == c.BrowserSocket {
		log.Fatal("distinct private listeners and service UID map required")
	}
	v, e := auth.New(c.Issuer, nil)
	if e != nil {
		log.Fatal(e)
	}
	g, e := gateway.New(c.Gateway, s, v)
	if e != nil {
		log.Fatal(e)
	}
	browser, e := ipc.Listen(c.BrowserSocket)
	if e != nil {
		log.Fatal(e)
	}
	defer browser.Close()
	service, e := ipc.Listen(c.ControlSocket)
	if e != nil {
		log.Fatal(e)
	}
	defer service.Close()
	bs, ss := ipc.Server(g), ipc.Server(s.ServiceHandler(c.ReadUIDs, c.ControllerUID, c.ReconcilerUID, c.PipelineUID))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go g.Streams.Run(ctx)
	go g.RunRevocations(ctx)
	fail := make(chan error, 2)
	go func() { fail <- bs.Serve(browser) }()
	go func() { fail <- ss.Serve(service) }()
	select {
	case e = <-fail:
		if !errors.Is(e, http.ErrServerClosed) {
			log.Print("gateway listener stopped")
			cancel()
		}
	case <-ctx.Done():
	}
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = bs.Shutdown(shutdown)
	_ = ss.Shutdown(shutdown)
}
