package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/tidwall/evio"
)

func runEvio() error {
	reuseport := flag.Bool("reuseport", false, "reuseport (SO_REUSEPORT)")
	trace := flag.Bool("trace", false, "print packets to console")
	loops := flag.Int("loops", 0, "num loops")
	stdlib := flag.Bool("stdlib", false, "use stdlib")
	flag.Parse()

	scheme := "tcp"
	if *stdlib {
		scheme += "-net"
	}
	addr := fmt.Sprintf("%s://%s:%d?reuseport=%t", scheme, *host, *port, *reuseport)

	return evio.Serve(evio.Events{
		NumLoops: *loops,
		Serving: func(srv evio.Server) (action evio.Action) {
			log.Printf("echo server started on port %d (loops: %d)", *port, srv.NumLoops)
			if *reuseport {
				log.Printf("reuseport")
			}
			if *stdlib {
				log.Printf("stdlib")
			}
			return
		},
		Data: func(c evio.Conn, in []byte) (out []byte, action evio.Action) {
			if *trace {
				log.Printf("%s", strings.TrimSpace(string(in)))
			}
			out = in
			return
		},
	}, addr)
}
