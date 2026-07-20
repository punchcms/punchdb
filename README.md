# punchdb

Database Layer

## tests

```bash
cd punchdb

# Tüm testler (verbose)
go test ./... -v

# Sadece pebble paketi, race detector ile (concurrency güvenliği)
go test ./pebble -race -v

# Benchmark'lar (alloc raporuyla)
go test ./pebble -bench=. -benchmem -run=^$ -benchtime=2s
```
