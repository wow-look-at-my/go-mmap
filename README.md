# go-mmap

Cross-platform memory-mapped file I/O for Go. Pure Go, no cgo.

Supports Linux, macOS, and Windows. Works with regular files and block devices.

## Install

```
go get github.com/wow-look-at-my/go-mmap
```

## Usage

### Map an entire file (read-only)

```go
m, err := mmap.MapFile("data.bin")
if err != nil {
    log.Fatal(err)
}
defer m.Unmap()

fmt.Println(string(m[:64])) // read first 64 bytes
```

### Map a region with read-write access

```go
f, _ := os.OpenFile("data.bin", os.O_RDWR, 0)
defer f.Close()

m, err := mmap.MapRegion(int(f.Fd()), 4096, mmap.ProtRead|mmap.ProtWrite, mmap.MapShared, 0)
if err != nil {
    log.Fatal(err)
}
defer m.Unmap()

copy(m, []byte("hello"))
m.Flush(mmap.SyncSync)
```

### Anonymous mapping (not backed by a file)

```go
m, err := mmap.MapRegion(-1, int64(os.Getpagesize()), mmap.ProtRead|mmap.ProtWrite, mmap.MapPrivate|mmap.MapAnonymous, 0)
if err != nil {
    log.Fatal(err)
}
defer m.Unmap()
```

### Reader (io.Reader, io.ReaderAt, io.WriterAt, io.Seeker, io.Closer)

```go
m, _ := mmap.MapFile("data.bin")
r := mmap.NewReader(m)
defer r.Close()

io.Copy(os.Stdout, r)
```

## API

| Function / Method | Description |
|---|---|
| `MapFile(path)` | Map entire file read-only |
| `MapRegion(fd, length, prot, flags, offset)` | Map a region with full control |
| `MMap.Unmap()` | Remove the mapping |
| `MMap.Flush(flags)` | Sync changes to disk |
| `MMap.Lock()` / `MMap.Unlock()` | Pin/unpin pages in RAM |
| `MMap.Advise(advice)` | Hint access pattern to kernel |

### Protection flags

`ProtRead`, `ProtWrite`, `ProtExec`

### Mapping flags

`MapShared`, `MapPrivate`, `MapAnonymous`

### Advise hints

`AdvNormal`, `AdvRandom`, `AdvSequential`, `AdvWillNeed`, `AdvDontNeed`

### Sync flags

`SyncAsync`, `SyncSync`, `SyncInvalidate`
