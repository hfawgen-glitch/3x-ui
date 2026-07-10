package amneziawg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// wgBinaryName is the userspace control tool used against both kernel and
// amneziawg-go interfaces: the kernel module and amneziawg-go both speak the
// same UAPI, so `awg` (an awg-quick-friendly rename of the wg tool) works
// against either backend identically once the interface exists.
func wgBinaryName() string { return "awg" }

func awgQuickBinaryName() string { return "awg-quick" }

func userspaceBinaryName() string { return "amneziawg-go" }

func configDir() string {
	return config.GetBinFolderPath() + "/amneziawg"
}

func configPathForID(id int) string {
	return fmt.Sprintf("%s/%s.conf", configDir(), ifaceNameForID(id))
}

// managed is the manager's bookkeeping for one running awg2 interface.
type managed struct {
	inst         Instance
	structuralFP string
	peersFP      string
	last         map[string]clientCounters
}

type clientCounters struct {
	up   int64
	down int64
}

// Traffic is a per-client traffic delta scraped from `wg show <iface>
// transfer`. Tag is the owning inbound's tag and Email is the client
// (matched by public key) the bytes belong to.
type Traffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

// Manager owns the set of running awg2 interfaces keyed by inbound id.
// Unlike MTProto's Manager (which owns opaque child *processes*), this
// Manager owns *network interfaces*: bringing one up/down is a synchronous
// external command (awg-quick), not a long-lived process this package
// supervises directly, so there is no analogue of mtproto.Process here.
type Manager struct {
	mu    sync.Mutex
	ifs   map[int]*managed
	swept bool
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide awg2 manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{ifs: map[int]*managed{}}
	})
	return manager
}

type ensureAction int

const (
	ensureNoop ensureAction = iota
	ensureReload
	ensureRestart
)

func ensureActionFor(exists bool, curStructFP, curPeersFP, newStructFP, newPeersFP string) ensureAction {
	if !exists || curStructFP != newStructFP {
		return ensureRestart
	}
	if curPeersFP != newPeersFP {
		return ensureReload
	}
	return ensureNoop
}

// Ensure brings an awg2 interface to the desired state, creating it if
// missing, hot-syncing peers in place if only the peer set changed, or doing
// a full down+up if any structural parameter changed.
func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	return m.ensureLocked(inst)
}

// sweepOrphansLocked tears down awg2 interfaces left up by a previous x-ui
// run (e.g. after a crash), exactly once per process lifetime. Anything
// matching the awg2-* naming scheme that this process does not yet know
// about is necessarily such an orphan, since every awg2 interface is
// created and named exclusively by this manager.
func (m *Manager) sweepOrphansLocked() {
	if m.swept {
		return
	}
	m.swept = true
	names, err := existingAwg2Interfaces()
	if err != nil {
		return
	}
	for _, name := range names {
		logger.Warningf("amneziawg: tearing down orphaned interface %s from a previous run", name)
		_ = runQuickDownByName(name)
	}
}

func (m *Manager) ensureLocked(inst Instance) error {
	structFP := inst.structuralFingerprint()
	peersFP := inst.peersFingerprint()

	if cur, ok := m.ifs[inst.Id]; ok {
		switch ensureActionFor(interfaceExists(cur.inst.IfaceName()), cur.structuralFP, cur.peersFP, structFP, peersFP) {
		case ensureNoop:
			cur.inst.Tag = inst.Tag
			return nil
		case ensureReload:
			if err := writeConfig(configPathForID(inst.Id), inst); err != nil {
				return err
			}
			if err := syncConf(inst); err == nil {
				m.ifs[inst.Id] = &managed{inst: inst, structuralFP: structFP, peersFP: peersFP, last: cur.last}
				logger.Infof("amneziawg: applied peer update to inbound %d in place", inst.Id)
				return nil
			}
			logger.Warningf("amneziawg: live peer sync unavailable for inbound %d, restarting interface", inst.Id)
			fallthrough
		case ensureRestart:
			_ = teardown(cur.inst)
			delete(m.ifs, inst.Id)
		}
	}

	if err := writeConfig(configPathForID(inst.Id), inst); err != nil {
		return err
	}
	if err := bringUp(inst); err != nil {
		return err
	}
	m.ifs[inst.Id] = &managed{inst: inst, structuralFP: structFP, peersFP: peersFP, last: map[string]clientCounters{}}
	logger.Infof("amneziawg: brought up %s for inbound %d (backend=%s)", inst.IfaceName(), inst.Id, inst.Backend)
	return nil
}

// Remove tears down and forgets the awg2 interface for an inbound id.
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.ifs[id]; ok {
		_ = teardown(cur.inst)
		delete(m.ifs, id)
		_ = os.Remove(configPathForID(id))
		logger.Infof("amneziawg: tore down %s for inbound %d", cur.inst.IfaceName(), id)
	}
}

// Reconcile drives the running set of interfaces toward the desired
// instances: tears down interfaces no longer wanted and (re)creates the
// rest. Used at boot and periodically to recover from a backend crash or an
// interface removed out-of-band.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	for id, cur := range m.ifs {
		if _, ok := want[id]; !ok {
			_ = teardown(cur.inst)
			delete(m.ifs, id)
			_ = os.Remove(configPathForID(id))
		}
	}
	for _, inst := range desired {
		if err := m.ensureLocked(inst); err != nil {
			logger.Warningf("amneziawg: reconcile failed for inbound %d: %v", inst.Id, err)
		}
	}
}

// StopAll tears down every managed awg2 interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.ifs {
		_ = teardown(cur.inst)
		_ = os.Remove(configPathForID(id))
		delete(m.ifs, id)
	}
}

func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cur := range m.ifs {
		if interfaceExists(cur.inst.IfaceName()) {
			return true
		}
	}
	return false
}

// CollectTraffic scrapes `wg show <iface> transfer` for each running
// interface and returns the per-client byte deltas since the previous
// scrape (matched by public key -> peer name), plus the emails of clients
// with nonzero live counters this scrape (i.e. currently active).
func (m *Manager) CollectTraffic() ([]Traffic, []string) {
	type snap struct {
		id    int
		iface string
		tag   string
		peers map[string]string // publicKey -> email
		last  map[string]clientCounters
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.ifs))
	for id, cur := range m.ifs {
		if !interfaceExists(cur.inst.IfaceName()) {
			continue
		}
		byKey := make(map[string]string, len(cur.inst.Peers))
		for _, p := range cur.inst.Peers {
			byKey[p.PublicKey] = p.Name
		}
		lastCopy := make(map[string]clientCounters, len(cur.last))
		maps.Copy(lastCopy, cur.last)
		snaps = append(snaps, snap{id: id, iface: cur.inst.IfaceName(), tag: cur.inst.Tag, peers: byKey, last: lastCopy})
	}
	m.mu.Unlock()

	var out []Traffic
	var online []string
	for _, s := range snaps {
		counters, ok := scrapeTransfer(s.iface)
		if !ok {
			continue
		}
		newLast := make(map[string]clientCounters, len(counters))
		for pubKey, c := range counters {
			email, known := s.peers[pubKey]
			if !known {
				continue
			}
			newLast[email] = c
			if c.up > 0 || c.down > 0 {
				online = append(online, email)
			}
			prev, had := s.last[email]
			if !had {
				continue
			}
			du, dd := c.up-prev.up, c.down-prev.down
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
			if du > 0 || dd > 0 {
				out = append(out, Traffic{Tag: s.tag, Email: email, Up: du, Down: dd})
			}
		}
		m.mu.Lock()
		if cur, ok := m.ifs[s.id]; ok {
			cur.last = newLast
		}
		m.mu.Unlock()
	}
	return out, online
}

// --- host interaction -------------------------------------------------

func writeConfig(path string, inst Instance) error {
	if err := os.MkdirAll(configDir(), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(renderConfig(inst)), 0o640)
}

// bringUp brings the interface up via the appropriate backend. For the
// kernel backend this is `awg-quick up <conf>`, which loads the amneziawg
// kernel module (if not already loaded), creates the interface, assigns
// addresses, and applies the WireGuard/AmneziaWG UAPI config in one step.
// For the userspace fallback, amneziawg-go creates the TUN device and this
// function then hands the same config to `awg setconf` to apply peers/keys,
// mirroring what awg-quick does internally for a kernel interface.
func bringUp(inst Instance) error {
	switch inst.Backend {
	case BackendUserspace:
		if err := startUserspace(inst); err != nil {
			return err
		}
		return applyConf(inst)
	default:
		return runCmd(awgQuickBinaryName(), "up", configPathForID(inst.Id))
	}
}

func teardown(inst Instance) error {
	if inst.Backend == BackendUserspace {
		stopUserspace(inst)
	}
	return runQuickDownByName(inst.IfaceName())
}

func runQuickDownByName(iface string) error {
	confPath := fmt.Sprintf("%s/%s.conf", configDir(), iface)
	if _, err := os.Stat(confPath); err == nil {
		return runCmd(awgQuickBinaryName(), "down", confPath)
	}
	// No config on disk (e.g. sweeping an orphan from a previous run whose
	// config was already cleaned up): fall back to a raw link delete so the
	// kernel interface does not linger.
	return runCmd("ip", "link", "delete", iface)
}

// startUserspace launches `amneziawg-go <iface>` in the background. Unlike
// the kernel module, this creates only the bare TUN device; addressing and
// UAPI config are applied afterward via applyConf, exactly matching how
// wireguard-go is bootstrapped by wg-quick on platforms without the kernel
// module.
func startUserspace(inst Instance) error {
	cmd := exec.CommandContext(context.Background(), userspaceBinaryName(), inst.IfaceName())
	cmd.Stdout = &awg2LogWriter{iface: inst.IfaceName()}
	cmd.Stderr = &awg2LogWriter{iface: inst.IfaceName()}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("amneziawg-go start failed: %w", err)
	}
	// amneziawg-go daemonizes itself and detaches (matching wireguard-go's
	// behavior), so unlike the mtg sidecar there is no long-lived *Process
	// to hold onto here; the manager tracks liveness via interfaceExists,
	// same as it does for the kernel backend.
	go cmd.Wait()
	return waitForInterface(inst.IfaceName(), 5*time.Second)
}

func stopUserspace(inst Instance) {
	// amneziawg-go removes its own TUN device on the interface being
	// deleted, same as wireguard-go; `ip link delete` is therefore the
	// correct stop for both backends.
	_ = runCmd("ip", "link", "delete", inst.IfaceName())
}

func waitForInterface(iface string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if interfaceExists(iface) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("interface %s did not appear within %s", iface, timeout)
}

// applyConf pushes the full instance config (address, private key, peers)
// onto an already-created TUN device via `wg setconf`, and assigns the
// interface addresses via `ip address add` — the two steps awg-quick does
// internally for a kernel interface, done manually here because
// amneziawg-go only creates the bare device.
func applyConf(inst Instance) error {
	if err := runCmd(wgBinaryName(), "setconf", inst.IfaceName(), configPathForID(inst.Id)); err != nil {
		return err
	}
	for _, addr := range inst.Address {
		if err := runCmd("ip", "address", "add", addr, "dev", inst.IfaceName()); err != nil {
			return err
		}
	}
	return runCmd("ip", "link", "set", "up", "dev", inst.IfaceName())
}

// syncConf hot-applies only the peer set of an already-up interface via `wg
// syncconf`, which adds/removes/re-keys peers without touching unrelated
// live connections — the awg2 analogue of mtg's PUT /secrets hot reload.
func syncConf(inst Instance) error {
	if !interfaceExists(inst.IfaceName()) {
		return errors.New("interface not up")
	}
	if err := writeConfig(configPathForID(inst.Id), inst); err != nil {
		return err
	}
	stripped, err := runCmdOutput(awgQuickBinaryName(), "strip", configPathForID(inst.Id))
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "awg2-sync-*.conf")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(stripped); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	return runCmd(wgBinaryName(), "syncconf", inst.IfaceName(), tmp.Name())
}

func interfaceExists(iface string) bool {
	_, err := os.Stat("/sys/class/net/" + iface)
	return err == nil
}

// existingAwg2Interfaces lists host interfaces matching the awg2-* naming
// scheme this manager exclusively uses, for orphan cleanup on startup.
func existingAwg2Interfaces() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "awg2-") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// scrapeTransfer parses `wg show <iface> transfer`, one line per peer:
// "<public-key>\t<rx-bytes>\t<tx-bytes>". rx is what the peer sent to the
// server (upload from the client's perspective) and tx is what the server
// sent back (download), matching the Up/Down convention used by every other
// protocol's traffic accounting in this panel.
func scrapeTransfer(iface string) (map[string]clientCounters, bool) {
	out, err := runCmdOutput(wgBinaryName(), "show", iface, "transfer")
	if err != nil {
		return nil, false
	}
	counters := map[string]clientCounters{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		rx, err1 := strconv.ParseInt(fields[1], 10, 64)
		tx, err2 := strconv.ParseInt(fields[2], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		counters[fields[0]] = clientCounters{up: rx, down: tx}
	}
	return counters, true
}

func runCmd(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// awg2LogWriter forwards amneziawg-go's stdout/stderr into the panel's log
// viewer with an `[awg2]`-style tag, exactly like mtg's own output is tagged
// via mtproto.procLogWriter, so both sidecars show up the same way in Logs.
type awg2LogWriter struct {
	iface string
	mu    sync.Mutex
	buf   string
}

func (w *awg2LogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(w.buf[:i])
		w.buf = w.buf[i+1:]
		if line != "" {
			logger.Infof("amneziawg: [awg2] %s | %s", w.iface, line)
		}
	}
	return len(p), nil
}
