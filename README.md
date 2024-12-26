# fastest (single-threaded) echo server ever

NOTE: benches done over localhost and WAN, 100 connections, 8 workers, see [Makefile](Makefile) for command.

## runGoro
- goroutines for each connection
- multi-threaded

```
Local:
Packet rate estimate: 1507283.6↓, 1502879.4↑ (8↓, 45↑ TCP MSS/op)
WAN:
Packet rate estimate: 6984.4↓, 8577.9↑ (2↓, 43↑ TCP MSS/op)
```

## runEpollReadNonblockingWriteBlocking
- single-threaded
- epoll for both read and writes over NONBLOCK sockets
- batch reads
- blocking writes, loop is used because either both read and write always block or both of them dont block, to batch reads we have to use nonblocking reads, so both are nonblocking

```
Local:
Packet rate estimate: 1117992.0↓, 1080027.2↑ (10↓, 45↑ TCP MSS/op)
WAN:
Packet rate estimate: 4635.3↓, 8570.7↑ (2↓, 44↑ TCP MSS/op)
```

## runEpollFullNonblocking !!!
- fastest ever
- single-threaded
- epoll for both read and writes over NONBLOCK sockets
- batch reads

I saw no implementations using both nonblocking reads and writes and batching reads. Batching over accept and reads are used to not go to epoll while there are ready data to read.

```
Local:
Packet rate estimate: 1658053.9↓, 1555625.9↑ (12↓, 45↑ TCP MSS/op)
WAN:
Packet rate estimate: 6772.0↓, 8709.6↑ (2↓, 43↑ TCP MSS/op)
```

## runEvio
- goroutine pool, each waiting on epoll

```
Local:
Packet rate estimate: 989894.2↓, 926918.0↑ (12↓, 45↑ TCP MSS/op)
WAN:
Packet rate estimate: 6816.3↓, 8626.2↑ (2↓, 43↑ TCP MSS/op)
```

## runGnet
- single or multiple threads, each waiting on epoll

```
Local:
Packet rate estimate: 2236104.5↓, 2094869.7↑ (12↓, 45↑ TCP MSS/op)
WAN:
Packet rate estimate: 7519.4↓, 8617.4↑ (2↓, 43↑ TCP MSS/op)
```