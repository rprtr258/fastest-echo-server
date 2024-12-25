# fastest (single-threaded) echo server ever

NOTE: benches done over localhost.

## runGoro
- goroutines for each connection
- multi-threaded

Bench:
```
Packet rate estimate: 232023.4↓, 177438.7↑ (1↓, 45↑ TCP MSS/op)
```

## runEpollReadNonblockingWriteBlocking
- single-threaded
- epoll for both read and writes over NONBLOCK sockets
- batch reads
- blocking writes, loop is used because either both read and write always block or both of them dont block, to batch reads we have to use nonblocking reads, so both are nonblocking

Bench:
```
Packet rate estimate: 1250023.9↓, 1233377.8↑ (11↓, 45↑ TCP MSS/op)
```

## runEpollFullNonblocking !!!
- fastest ever
- single-threaded
- epoll for both read and writes over NONBLOCK sockets
- batch reads

I saw no implementations using both nonblocking reads and writes and batching reads. Batching over accept and reads are used to not go to epoll while there are ready data to read.

Bench:
```
Packet rate estimate: 1433477.7↓, 1413431.4↑ (11↓, 45↑ TCP MSS/op)
```
