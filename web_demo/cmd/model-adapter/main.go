// model-adapter is installed in the network-none participant image.
package main

import (
	"flag"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/broker"
	"log"
)

func main() {
	initialize := flag.String("initialize-cert", "", "explicitly create scoped certificates in an existing private workspace directory")
	address := flag.String("listen", "127.0.0.1:8443", "HTTPS IPv4 loopback address")
	socket := flag.String("socket", "", "mounted workspace broker socket")
	cert := flag.String("cert", "", "scoped loopback TLS certificate")
	key := flag.String("key", "", "workspace TLS private key")
	capabilityFile := flag.String("capability-file", "", "current read-only controller-provisioned workspace capability")
	flag.Parse()
	if *initialize != "" {
		if err := broker.InitializeLoopbackTrust(*initialize); err != nil {
			log.Fatal(err)
		}
		return
	}
	h, err := broker.Adapter(*socket)
	if *capabilityFile != "" {
		h, err = broker.AdapterWithCapability(*socket, *capabilityFile)
	}
	if err != nil {
		log.Fatal(err)
	}
	s, l, err := broker.LoopbackTLS(*address, *cert, *key, h)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(s.Serve(l))
}
