// pipeline is the private operator worker. It has no public listener or shell API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, `{"protocol":"feam.pipeline.v1","error":"operation_failed; inspect recorded status and reconcile before retry"}`)
		os.Exit(1)
	}
}
func execute() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("missing explicit action")
	}
	action := os.Args[1]
	f := flag.NewFlagSet("pipeline "+action, flag.ContinueOnError)
	root := f.String("root", "", "existing dedicated pipeline root")
	feam := f.String("feam", "/usr/local/bin/feam", "pinned installed FEAM executable")
	python := f.String("python", "/opt/feam/pipeline/bin/python", "pinned converter interpreter")
	converter := f.String("converter", "/opt/feam/importers/convert.py", "installed fixed converter")
	job := f.String("job", "", "private approved job file")
	inbox := f.String("inbox", "", "private operator source-ID directory; empty permits approved bounded fetch")
	id := f.String("id", "", "existing durable job UUID")
	hash := f.String("hash", "", "exact approved job or candidate SHA-256")
	actor := f.String("actor", "", "operator-recorded approving admin UUID")
	socket := f.String("socket", "", "private controller/gateway Unix socket")
	controllerUID := f.Uint("controller-uid", 0, "installed controller service UID")
	gatewayUID := f.Uint("gateway-uid", 0, "installed gateway service UID; zero disables gateway reads")
	webDemoACL := f.Bool("web-demo-acl", false, "Linux fixed web-demo serving-reader ACLs on approved release promotion")
	controlSocket := f.String("control-socket", "", "private gateway authority socket for withdrawal and retained-release pins")
	if err := f.Parse(os.Args[2:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	var spec pipeline.Job
	if action == "hash" || action == "prepare" {
		b, err := os.ReadFile(*job)
		if err != nil {
			return err
		}
		spec, err = pipeline.DecodeJob(b)
		if err != nil {
			return err
		}
		if action == "hash" {
			fmt.Println(spec.Hash())
			return nil
		}
	}
	if *root == "" || !filepath.IsAbs(*root) {
		return fmt.Errorf("absolute root required")
	}
	c := pipeline.DefaultConfig(*root, *feam, *python, *converter)
	c.WebDemoACL = *webDemoACL
	c.ControlSocket = *controlSocket
	var manager *pipeline.Manager
	var err error
	if action == "initialize" {
		manager, err = pipeline.Initialize(c)
	} else if action == "migrate" {
		manager, err = pipeline.Migrate(c)
	} else {
		manager, err = pipeline.Open(c)
	}
	if err != nil {
		return err
	}
	defer manager.DB.Close()
	if action == "serve" {
		if *controllerUID == 0 || *controllerUID == *gatewayUID || uint64(*controllerUID) > uint64(^uint32(0)) || uint64(*gatewayUID) > uint64(^uint32(0)) {
			return fmt.Errorf("nonroot controller UID required")
		}
		listener, err := ipc.Listen(*socket)
		if err != nil {
			return err
		}
		defer listener.Close()
		server := ipc.Server(manager.ServiceHandler(uint32(*controllerUID), uint32(*gatewayUID)))
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		queueDone := make(chan struct{})
		go func() { defer close(queueDone); manager.RunQueue(ctx) }()
		defer func() { cancel(); <-queueDone }()
		go func() {
			<-ctx.Done()
			shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			server.Shutdown(shutdown)
		}()
		err = server.Serve(listener)
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var result any
	switch action {
	case "initialize":
		result = map[string]string{"protocol": pipeline.Protocol, "status": "initialized"}
	case "migrate":
		result = map[string]string{"protocol": pipeline.Protocol, "status": "migrated"}
	case "prepare":
		result, err = manager.Prepare(ctx, spec, *hash, *inbox)
	case "status":
		result, err = manager.Status(*id)
	case "approve":
		err = manager.Approve(*id, *hash, *actor)
		if err == nil {
			result, err = manager.Status(*id)
		}
	case "promote":
		result, err = manager.Promote(*id)
	case "reconcile":
		result, err = manager.Reconcile(ctx, *id)
	default:
		return fmt.Errorf("unknown fixed action")
	}
	if result != nil {
		if e := json.NewEncoder(os.Stdout).Encode(result); e != nil {
			return e
		}
	}
	return err
}
