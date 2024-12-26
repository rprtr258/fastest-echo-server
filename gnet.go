package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/panjf2000/gnet/v2"
)

type echoServer struct {
	gnet.BuiltinEventEngine

	eng       gnet.Engine
	addr      string
	multicore bool
}

func (es *echoServer) OnBoot(eng gnet.Engine) gnet.Action {
	es.eng = eng
	log.Printf("echo server with multi-core=%t is listening on %s\n", es.multicore, es.addr)
	return gnet.None
}

func (es *echoServer) OnTraffic(c gnet.Conn) gnet.Action {
	buf, _ := c.Next(-1)
	c.Write(buf)
	return gnet.None
}

func runGnet() error {
	multicore := flag.Bool("multicore", false, "--multicore true")
	flag.Parse()

	echo := &echoServer{addr: fmt.Sprintf("tcp://:%d", *port), multicore: *multicore}
	return gnet.Run(echo, echo.addr, gnet.WithMulticore(*multicore))
}
