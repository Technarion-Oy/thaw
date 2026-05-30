# internal/sysinfo

> Lightweight host system introspection — currently exposes total physical RAM.

## Responsibility

Provides a single helper, `MemoryGB()`, that queries the host OS for total physical memory.
The value is used by the AI settings UI to suggest a sensible Ollama context-window size
(larger models benefit from more RAM for the KV cache).

## Key files

| File | Purpose |
|---|---|
| `sysinfo.go` | `MemoryGB()` implementation via `sysctl`. |

## Key types & functions

| Symbol | Description |
|---|---|
| `MemoryGB() int` | Returns total physical RAM in GB (rounded down). Returns `0` on unsupported platforms or if the value cannot be parsed. Tries `hw.memsize` (macOS) then `hw.physmem` (some BSDs) via `sysctl -n`. |

## Patterns & integration

- Called from `internal/app/config.go` (or the AI settings IPC method) and returned to the frontend so the Ollama configuration modal can pre-populate a reasonable context size.
- Requires `sysctl` to be available on PATH (standard on macOS and Linux). On Windows, `sysctl` is not available and `MemoryGB()` returns `0`.

## Gotchas

- Windows is not supported by the current `sysctl`-based implementation. A `syscall`-based Windows alternative (e.g. `GlobalMemoryStatusEx`) would be needed to support it.
- The function runs `sysctl` as a subprocess, so it is slightly slower than a direct syscall. It is only called during settings UI interactions, not on hot paths.
