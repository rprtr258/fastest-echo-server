package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/felixge/fgprof"
	"golang.org/x/sys/unix"
)

var (
	bufsize = flag.Int("bufsize", 4096, "Buffer size to allocate for each read.")
	port    = flag.Int("port", 3333, "Port to accept connections on.")
	host    = flag.String("host", "0.0.0.0", "Host or IP to bind to")
)

func runEpollReadNonblockingWriteBlocking() error {
	addr, err := netip.ParseAddrPort(*host + ":" + strconv.Itoa(*port))
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addr))
	if err != nil {
		return err
	}
	log.Printf("Listening to connections at %s:%d\n", *host, *port)
	defer l.Close()

	type netFD struct {
		pfd struct {
			fdmu struct {
				state uint64
				rsema uint32
				wsema uint32
			}
			Sysfd   int
			SysFile struct {
				iovecs *[]syscall.Iovec
			}
			pd struct {
				runtimeCtx uintptr
			}
			csema         uint32
			isBlocking    uint32
			IsStream      bool
			ZeroReadIsEOF bool
			isFile        bool
		}
		// immutable until Close
		family      int
		sotype      int
		isConnected bool // handshake completed or use of association with peer
		net         string
		laddr       net.Addr
		raddr       net.Addr
	}
	srvFD := (*struct {
		fd *netFD
		lc net.ListenConfig
	})(unsafe.Pointer(l)).fd.pfd.Sysfd
	if _, err := unix.FcntlInt(uintptr(srvFD), unix.F_SETFL, unix.O_NONBLOCK); err != nil {
		return fmt.Errorf("fcntl: %w", err)
	}
	buf := make([]byte, *bufsize)
	events := []unix.PollFd{{int32(srvFD), unix.EPOLLIN, 0}}
	for {
		// fmt.Println(events)
		if _, err := unix.Poll(events, -1); err != nil {
			return fmt.Errorf("poll: %#v", err)
		}
		// fmt.Println(events)

		// TODO: is it x/sys/unix leaves us with events slice or we really reuse it all the way along?
		if events[0].Revents&unix.POLLIN != 0 {
			for {
				nfd, sa, err := syscall.Accept4(srvFD, 0)
				if err != nil {
					if err == syscall.EWOULDBLOCK {
						break
					}
					return fmt.Errorf("accept: %w", err)
				}
				if _, err := unix.FcntlInt(uintptr(nfd), unix.F_SETFL, unix.O_NONBLOCK); err != nil {
					return fmt.Errorf("fcntl: %w", err)
				}
				_ = sa // log.Println("Accepted new connection", sa)
				// defer conn.Close()

				events = append(events, unix.PollFd{int32(nfd), unix.POLLIN, 0})
				events[0].Revents &^= unix.POLLIN
			}
		}
		for i := 1; i < len(events); i++ {
			fd := events[i].Fd
			if events[i].Revents&unix.POLLIN != 0 {
				nn := 0
				for len(buf[nn:]) > 0 {
					n, err := syscall.Read(int(fd), buf[nn:])
					if err != nil {
						// log.Printf("read %d %#v\n", fd, err)
						if err == syscall.EWOULDBLOCK {
							break
						}
						return fmt.Errorf("read: %w", err)
					} else if n == 0 {
						goto CLOSE
					}
					// log.Println("Read new data from connection", string(buf[fd][nn[fd]:][:n]))
					nn += n
				}

				data := buf[:nn]
				for { // NOTE: stupid spin loop on nonblocking write
					if n, err := syscall.Write(int(fd), data); err != nil {
						// log.Printf("write %d %#v\n", fd, err)
						if err == syscall.EWOULDBLOCK {
							continue
						}
						if err == syscall.ECONNRESET {
							goto CLOSE
						}
						return fmt.Errorf("write: %w", err)
					} else if n == 0 {
						goto CLOSE
					}
					break
				}

				events[i].Events |= unix.POLLIN
				events[i].Revents &^= unix.POLLIN
			}
			if events[i].Revents != 0 {
				// unprocessed events left
				return fmt.Errorf("revents: %#v", events[i].Revents)
			}
			continue
		CLOSE:
			// log.Println("Closed connection")
			events = append(events[:i], events[i+1:]...)
			i--
		}
	}
}

func runEpollFullNonblocking() error {
	addr, err := netip.ParseAddrPort(*host + ":" + strconv.Itoa(*port))
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addr))
	if err != nil {
		return err
	}
	log.Printf("Listening to connections at %s:%d\n", *host, *port)
	defer l.Close()

	type netFD struct {
		pfd struct {
			fdmu struct {
				state uint64
				rsema uint32
				wsema uint32
			}
			Sysfd   int
			SysFile struct {
				iovecs *[]syscall.Iovec
			}
			pd struct {
				runtimeCtx uintptr
			}
			csema         uint32
			isBlocking    uint32
			IsStream      bool
			ZeroReadIsEOF bool
			isFile        bool
		}
		// immutable until Close
		family      int
		sotype      int
		isConnected bool // handshake completed or use of association with peer
		net         string
		laddr       net.Addr
		raddr       net.Addr
	}
	freeblocks := [][]byte{}
	bufs := [][][]byte{} // for each conn, for each read, buffer we have read in single syscall
	srvFD := (*struct {
		fd *netFD
		lc net.ListenConfig
	})(unsafe.Pointer(l)).fd.pfd.Sysfd
	if _, err := unix.FcntlInt(uintptr(srvFD), unix.F_SETFL, unix.O_NONBLOCK); err != nil {
		return fmt.Errorf("fcntl: %w", err)
	}
	events := []unix.PollFd{{int32(srvFD), unix.EPOLLIN, 0}}
	debug := func(dir string) {
		log := func(s ...any) { fmt.Fprint(os.Stderr, s...) }
		log(dir, " EVENTS ", events, " FREEBLOCKS ", len(freeblocks), " [")
		for i := range bufs {
			if i != 0 {
				log(" ")
			}
			log("[")
			for j := range bufs[i] {
				if j != 0 {
					log(" ")
				}
				log(len(bufs[i][j]))
			}
			log("]")
		}
		log("]\n")
	}
	_ = debug
	for {
		// debug(">")
		if _, err := unix.Ppoll(events, nil, nil); err != nil {
			return fmt.Errorf("poll: %#v", err)
		}
		// debug("<")

		// TODO: is it x/sys/unix leaves us with events slice or we really reuse it all the way along?
		if events[0].Revents&unix.POLLIN != 0 {
			for {
				nfd, sa, err := syscall.Accept4(srvFD, 0)
				if err != nil {
					if err == syscall.EWOULDBLOCK {
						break
					}
					return fmt.Errorf("accept: %w", err)
				}
				if _, err := unix.FcntlInt(uintptr(nfd), unix.F_SETFL, unix.O_NONBLOCK); err != nil {
					return fmt.Errorf("fcntl: %w", err)
				}
				_ = sa // log.Println("Accepted new connection", sa)
				// defer conn.Close()

				if len(bufs) == len(events)-1 {
					bufs = append(bufs, [][]byte{})
				}
				events = append(events, unix.PollFd{int32(nfd), unix.POLLIN, 0})
				events[0].Revents &^= unix.POLLIN
			}
		}
		for i := 1; i < len(events); i++ {
			fd := events[i].Fd
			// TODO: read before write handling does not work, why?
			if events[i].Revents&unix.POLLOUT != 0 {
				if n, err := unix.Writev(int(fd), bufs[i-1]); err != nil {
					// debug("=")
					// log.Printf("write %d %#v\n", fd, err)
					if err == syscall.ECONNRESET {
						goto CLOSE
					}
					return fmt.Errorf("write: %w", err)
				} else if n == 0 {
					goto CLOSE
				}
				freeblocks = append(freeblocks, bufs[i-1]...)
				bufs[i-1] = bufs[i-1][:0]
				events[i].Events &^= unix.POLLOUT
				events[i].Revents &^= unix.POLLOUT
			}
			if events[i].Revents&unix.POLLIN != 0 {
				const _maxreads = 8 // max reads per conn

				for len(bufs[i-1]) < _maxreads {
					var buf []byte
					if k := len(freeblocks); k > 0 {
						buf = freeblocks[k-1]
						freeblocks = freeblocks[:k-1]
					} else {
						buf = make([]byte, *bufsize)
					}

					n, err := syscall.Read(int(fd), buf[:*bufsize])
					if err != nil {
						freeblocks = append(freeblocks, buf)
						// log.Printf("read %d %#v\n", fd, err)
						if err == syscall.EWOULDBLOCK {
							break
						}
						return fmt.Errorf("read: %w", err)
					} else if n == 0 {
						freeblocks = append(freeblocks, buf)
						goto CLOSE
					}
					// log.Println("Read new data from connection", string(buf[:n]))
					bufs[i-1] = append(bufs[i-1], buf[:n])
				}
				events[i].Revents &^= unix.POLLIN
				events[i].Events |= unix.POLLOUT
				if len(bufs[i-1]) < _maxreads {
					events[i].Events |= unix.POLLIN
				}
			}
			if events[i].Revents != 0 {
				// unprocessed events left
				return fmt.Errorf("revents: %#v", events[i].Revents)
			}
			continue
		CLOSE:
			// log.Println("Closed connection")
			events = append(events[:i], events[i+1:]...)
			freeblocks = append(freeblocks, bufs[i-1]...)
			bufs = append(bufs[:i-1], bufs[i:]...)
			i--
		}
	}
}

func runGoro() error {
	l, err := net.Listen("tcp", *host+":"+strconv.Itoa(*port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	log.Println("Listening to connections at '"+*host+"' on port", strconv.Itoa(*port))
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		go func() {
			// log.Println("Accepted new connection.")

			buf := make([]byte, *bufsize)
			for {
				size, err := conn.Read(buf)
				if err != nil {
					// log.Println("Closed connection.")
					conn.Close()
					return
				}
				data := buf[:size]
				// log.Println("Read new data from connection", data)
				conn.Write(data)
			}
		}()
	}
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	f, err := os.Create("fgprof.pprof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	stop := fgprof.Start(f, fgprof.FormatPprof)
	go func() {
		<-time.After(time.Second * 10)
		stop()
	}()

	if err :=
		// runGoro();
		// runEpollReadNonblockingWriteBlocking();
		runEpollFullNonblocking();
	//
	err != nil {
		log.Println(err.Error())
	}
}
