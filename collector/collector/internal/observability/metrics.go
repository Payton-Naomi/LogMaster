package observability

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Registry struct {
	AgentID string
	Version string

	mu              sync.RWMutex
	serialConnected map[string]bool
	serialRXBytes   map[string]uint64
	serialReconnect map[string]uint64
	spoolBatches    map[string]int64
	spoolBytes      map[string]int64
	uploads         map[string]uint64
	analyses        map[string]uint64
	diskFreeBytes   atomic.Int64
}

func NewRegistry(agentID, version string) *Registry {
	return &Registry{AgentID: agentID, Version: version, serialConnected: map[string]bool{}, serialRXBytes: map[string]uint64{}, serialReconnect: map[string]uint64{}, spoolBatches: map[string]int64{}, spoolBytes: map[string]int64{}, uploads: map[string]uint64{}, analyses: map[string]uint64{}}
}

func (r *Registry) SetSerialConnected(device, port string, connected bool) {
	r.mu.Lock()
	r.serialConnected[device+"\x00"+port] = connected
	r.mu.Unlock()
}

func (r *Registry) AddSerialBytes(device string, n uint64) {
	r.mu.Lock()
	r.serialRXBytes[device] += n
	r.mu.Unlock()
}

func (r *Registry) IncReconnect(device, reason string) {
	r.mu.Lock()
	r.serialReconnect[device+"\x00"+reason]++
	r.mu.Unlock()
}

func (r *Registry) SetSpool(state string, batches, bytes int64) {
	r.mu.Lock()
	r.spoolBatches[state], r.spoolBytes[state] = batches, bytes
	r.mu.Unlock()
}

func (r *Registry) IncUpload(result string) {
	r.mu.Lock()
	r.uploads[result]++
	r.mu.Unlock()
}

func (r *Registry) IncAnalysis(provider, result string) {
	r.mu.Lock()
	r.analyses[provider+"\x00"+result]++
	r.mu.Unlock()
}

func (r *Registry) SetDiskFreeBytes(value int64) { r.diskFreeBytes.Store(value) }

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(r.render()))
	})
}

func (r *Registry) render() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out strings.Builder
	fmt.Fprintf(&out, "logmaster_agent_info{version=%s,agent_id=%s} 1\n", quote(r.Version), quote(r.AgentID))
	for _, key := range sortedKeys(r.serialConnected) {
		parts := strings.SplitN(key, "\x00", 2)
		value := 0
		if r.serialConnected[key] {
			value = 1
		}
		fmt.Fprintf(&out, "logmaster_serial_connected{device_sn=%s,port_name=%s} %d\n", quote(parts[0]), quote(parts[1]), value)
	}
	for _, key := range sortedKeys(r.serialRXBytes) {
		fmt.Fprintf(&out, "logmaster_serial_rx_bytes_total{device_sn=%s} %d\n", quote(key), r.serialRXBytes[key])
	}
	for _, key := range sortedKeys(r.serialReconnect) {
		parts := strings.SplitN(key, "\x00", 2)
		fmt.Fprintf(&out, "logmaster_serial_reconnect_total{device_sn=%s,reason=%s} %d\n", quote(parts[0]), quote(parts[1]), r.serialReconnect[key])
	}
	for _, key := range sortedKeys(r.spoolBatches) {
		fmt.Fprintf(&out, "logmaster_spool_batches{state=%s} %d\n", quote(key), r.spoolBatches[key])
		fmt.Fprintf(&out, "logmaster_spool_bytes{state=%s} %d\n", quote(key), r.spoolBytes[key])
	}
	for _, key := range sortedKeys(r.uploads) {
		fmt.Fprintf(&out, "logmaster_upload_total{result=%s} %d\n", quote(key), r.uploads[key])
	}
	for _, key := range sortedKeys(r.analyses) {
		parts := strings.SplitN(key, "\x00", 2)
		fmt.Fprintf(&out, "logmaster_analysis_total{provider=%s,result=%s} %d\n", quote(parts[0]), quote(parts[1]), r.analyses[key])
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	fmt.Fprintf(&out, "logmaster_process_memory_bytes %d\n", memory.Alloc)
	fmt.Fprintf(&out, "logmaster_disk_free_bytes %d\n", r.diskFreeBytes.Load())
	return out.String()
}

func quote(value string) string { return strconv.Quote(value) }

func sortedKeys[V any](input map[string]V) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
