// private-terminal is a W1 operator tool, never the production Access gateway.
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/terminal"
)

func main() {
	listen := flag.String("socket", "", "new private Unix listener; existing paths are never removed")
	backend := flag.String("terminal-socket", "", "fixed workspace ttyd Unix socket")
	origin := flag.String("origin", "http://127.0.0.1:18771", "exact private SSH-forwarded browser origin")
	key := flag.String("credential-file", "", "owner-only file with a 64-character random hex credential")
	flag.Parse()
	if flag.NArg() != 0 || !strings.HasPrefix(*listen, "/") || *listen == *backend {
		log.Fatal("private Unix socket arguments required")
	}
	info, err := os.Lstat(*key)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		log.Fatal("credential must be a private regular file")
	}
	data, err := os.ReadFile(*key)
	if err != nil {
		log.Fatal("cannot read operator credential")
	}
	proxy, err := terminal.New(*origin, *backend, strings.TrimSpace(string(data)))
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("unix", *listen)
	if err != nil {
		log.Fatal("cannot create private listener")
	}
	defer listener.Close()
	if err := os.Chmod(*listen, 0600); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Print("W1 private operator terminal ready; SSH transport required")
	log.Fatal(server.Serve(listener))
}
