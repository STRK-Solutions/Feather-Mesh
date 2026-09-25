package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/broker"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/collector"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

type config struct {
	Ledger               string `json:"ledger"`
	Capabilities         string `json:"capabilities"`
	Allocation           string `json:"allocation"`
	PublicKey            string `json:"operator_public_key"`
	DeploymentID         string `json:"deployment_id"`
	ActivationGeneration int64  `json:"activation_generation"`
	AuthoritySocket      string `json:"authority_socket"`
	ControlSocket        string `json:"control_socket"`
	ControllerSocket     string `json:"controller_socket"`
	ReceiptFile          string `json:"receipt_file"`
	ControllerUID        uint32 `json:"controller_uid"`
	GatewayUID           uint32 `json:"gateway_uid"`
	UpstreamKeyFile      string `json:"upstream_key_file"`
	CollectorSocket      string `json:"collector_socket"`
	SoftwareSHA256       string `json:"software_sha256"`
	ProfileSHA256        string `json:"profile_sha256"`
	DatasetSHA256        string `json:"dataset_sha256"`
	Sockets              []struct {
		WorkspaceID string `json:"workspace_id"`
		Path        string `json:"path"`
	} `json:"sockets"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	path := flag.String("config", "", "private broker config JSON")
	initialize := flag.Bool("initialize", false, "explicitly initialize new local run and capability state")
	fake := flag.Bool("fake-provider", false, "explicit synthetic offline provider")
	live := flag.Bool("approved-live-provider", false, "operator-authorized finite live dispatch (never tests)")
	flag.Parse()
	var c config
	b, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	if broker.StrictDecode(b, &c) != nil || !budget.ValidID(c.DeploymentID) || c.ActivationGeneration < 1 || c.ControllerUID == 0 || c.GatewayUID == 0 || c.GatewayUID == c.ControllerUID ||
		!filepath.IsAbs(c.ReceiptFile) || filepath.Clean(c.ReceiptFile) != c.ReceiptFile || filepath.Dir(c.ReceiptFile) != filepath.Dir(c.Ledger) ||
		c.ReceiptFile == c.Ledger || c.ReceiptFile == c.Capabilities {
		return budget.ErrInvalid
	}
	var doc budget.SignedAllocation
	b, err = os.ReadFile(c.Allocation)
	if err != nil {
		return err
	}
	if err = broker.StrictDecode(b, &doc); err != nil {
		return err
	}
	pub, err := budget.ReadPrivateKey(c.PublicKey, ed25519.PublicKeySize)
	if err != nil {
		return err
	}
	var ledger *budget.Run
	var caps *capability.Store
	if *initialize {
		ledger, err = budget.InitializeRun(c.Ledger, doc, pub, c.DeploymentID, c.ActivationGeneration)
		if err != nil {
			return err
		}
		defer ledger.Close()
		caps, err = capability.Initialize(c.Capabilities)
		if err != nil {
			return err
		}
		return caps.DB.Close()
	}
	if *fake == *live {
		return errors.New("select exactly one explicit provider mode")
	}
	ledger, err = budget.OpenRun(c.Ledger, doc, pub, c.DeploymentID, c.ActivationGeneration)
	if err != nil {
		return err
	}
	defer ledger.Close()
	caps, err = capability.Open(c.Capabilities)
	if err != nil {
		return err
	}
	defer caps.DB.Close()
	var p broker.Provider = broker.FakeProvider{}
	secret := ""
	if *live {
		fi, err := os.Lstat(c.UpstreamKeyFile)
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 4096 {
			return errors.New("upstream key file must be bounded and private")
		}
		b, err := os.ReadFile(c.UpstreamKeyFile)
		if err != nil {
			return err
		}
		secret = string(b)
		p, err = broker.NewOpenRouter(secret)
		if err != nil {
			return err
		}
	}
	s := broker.NewServer(ledger, caps, broker.NewControlAuthority(c.AuthoritySocket), p)
	s.Secret = secret
	s.ReceiptFile = c.ReceiptFile
	if c.ControllerSocket != "" {
		s.Operations = broker.NewControlOperations(c.ControllerSocket)
	} else if *live {
		return errors.New("live admission requires controller activity accounting")
	}
	if c.CollectorSocket != "" {
		recorder, err := collector.NewBrokerRecorder(c.CollectorSocket, c.SoftwareSHA256, c.ProfileSHA256, c.DatasetSHA256)
		if err != nil {
			return err
		}
		s.Events = recorder
	} else if *live {
		return errors.New("recorded live admission requires the configured collector")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, s, c)
}

func serve(ctx context.Context, s *broker.Server, c config) error {
	if len(c.Sockets) == 0 || len(c.Sockets) > 12 {
		return budget.ErrInvalid
	}
	seen := map[string]bool{}
	for _, endpoint := range c.Sockets {
		if !budget.ValidID(endpoint.WorkspaceID) || seen[endpoint.WorkspaceID] {
			return budget.ErrInvalid
		}
		seen[endpoint.WorkspaceID] = true
	}
	var listeners []net.Listener
	var servers []*http.Server
	control, err := ipc.Listen(c.ControlSocket)
	if err != nil {
		return err
	}
	defer control.Close()
	listeners = append(listeners, control)
	controlServer := ipc.Server(s.Control(c.ControllerUID, c.GatewayUID))
	controlServer.WriteTimeout = 35 * time.Second
	servers = append(servers, controlServer)
	for _, endpoint := range c.Sockets {
		l, err := ipc.Listen(endpoint.Path)
		if err != nil {
			return err
		}
		defer l.Close()
		server := &http.Server{Handler: s.Workspace(endpoint.WorkspaceID), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 125 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10}
		listeners = append(listeners, l)
		servers = append(servers, server)
	}
	failures := make(chan error, len(servers))
	for i, server := range servers {
		defer server.Close()
		go func() { failures <- server.Serve(listeners[i]) }()
	}
	var failure error
	select {
	case <-ctx.Done():
	case failure = <-failures:
	}
	drain, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, drainErr := s.Shutdown(drain)
	cancel()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func() { defer wg.Done(); _ = server.Shutdown(shutdown) }()
	}
	wg.Wait()
	return errors.Join(failure, drainErr)
}
