// Package amneziawg manages AmneziaWG 2.0 sidecar interfaces. Xray-core's
// "wireguard" proxy links wireguard-go, an unmodified userspace WireGuard
// implementation that cannot speak the AmneziaWG obfuscated wire format
// (junk packets Jc/Jmin/Jmax, header magic S1-S4/H1-H4, extra init-packet
// padding I1-I5) — that is a different transport framing, not an extra JSON
// field. So, exactly like MTProto (see internal/mtproto), an awg2 inbound is
// not run through Xray at all: the panel manages a real AmneziaWG network
// interface as a standalone sidecar (kernel module via awg-quick, or the
// amneziawg-go userspace binary as a fallback), entirely outside the Xray
// config and lifecycle. In the UI, database, QR codes, traffic accounting
// and logs it behaves like every other protocol; only the transport is
// external.
package amneziawg

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Backend selects which AmneziaWG implementation actually moves packets for
// an interface.
type Backend string

const (
	// BackendKernel drives a real amneziawg kernel module through awg-quick,
	// the default and best-performing option on hosts where the module can
	// be loaded (bare metal / most KVM guests).
	BackendKernel Backend = "kernel"
	// BackendUserspace runs the amneziawg-go userspace implementation
	// (MIT-licensed drop-in replacement for wireguard-go), used as a
	// fallback where a kernel module cannot be loaded — LXC containers,
	// Secure Boot hosts that block unsigned modules, some managed VPS
	// kernels, etc.
	BackendUserspace Backend = "userspace"
)

// Peer is one client attached to an awg2 interface.
type Peer struct {
	Name         string // client email; used as the wg-conf/log identity
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
	KeepAlive    int
	QuotaBytes   int64
	ExpiresUnix  int64
}

// Instance is the desired runtime configuration of one awg2 inbound. Unlike
// MTProto (multi-secret, single process, no concept of per-peer IP), an awg2
// instance is a single network interface with one address pool and N peers,
// which maps directly onto how awg-quick / amneziawg-go expect the config
// file to look: one [Interface] section, one [Peer] section per client.
type Instance struct {
	Id      int
	Tag     string
	Ifname  string // interface name on the host, derived from Id (see IfaceName)
	Backend Backend

	PrivateKey string
	Address    []string // interface CIDRs, v4 and/or v6
	ListenPort int
	MTU        int

	// AmneziaWG 2.0 obfuscation parameters. Zero-valued fields are omitted
	// from the generated config so the backend falls back to its own
	// defaults (or, for Jmin/Jmax/Jc = 0, plain unobfuscated WireGuard
	// framing) rather than silently forcing zeros.
	Jc, Jmin, Jmax     int
	S1, S2, S3, S4     int
	H1, H2, H3, H4     uint32
	I1, I2, I3, I4, I5 string

	Peers []Peer
}

func ifaceNameForID(id int) string {
	// Linux interface names are capped at 15 bytes; "awg2-" + id comfortably
	// fits for any realistic inbound id and stays greppable in `ip link`.
	return fmt.Sprintf("awg2-%d", id)
}

// IfaceName returns the host network interface name for this instance.
func (inst Instance) IfaceName() string {
	if inst.Ifname != "" {
		return inst.Ifname
	}
	return ifaceNameForID(inst.Id)
}

// structuralFingerprint changes whenever a value that requires tearing down
// and re-creating the network interface changes (address, port, MTU,
// obfuscation parameters, backend, the server's own private key). Unlike
// mtg, awg-quick has no in-place "reconfigure the interface itself" path
// short of `wg-quick down && up`, so any of these forces a full restart.
func (inst Instance) structuralFingerprint() string {
	parts := []string{
		string(inst.Backend),
		inst.PrivateKey,
		strings.Join(inst.Address, ","),
		strconv.Itoa(inst.ListenPort),
		strconv.Itoa(inst.MTU),
		strconv.Itoa(inst.Jc), strconv.Itoa(inst.Jmin), strconv.Itoa(inst.Jmax),
		strconv.Itoa(inst.S1), strconv.Itoa(inst.S2), strconv.Itoa(inst.S3), strconv.Itoa(inst.S4),
		strconv.FormatUint(uint64(inst.H1), 10), strconv.FormatUint(uint64(inst.H2), 10),
		strconv.FormatUint(uint64(inst.H3), 10), strconv.FormatUint(uint64(inst.H4), 10),
		inst.I1, inst.I2, inst.I3, inst.I4, inst.I5,
	}
	return strings.Join(parts, "|")
}

// peersFingerprint identifies the reloadable peer set regardless of order, so
// a reordered clients array in the stored settings does not read as a
// change. `wg syncconf` (which awg-quick's strip/syncconf machinery is built
// on) can add, remove, or re-key peers on a live interface without dropping
// unrelated connections, so a peers-only change is a reload candidate rather
// than a restart.
func (inst Instance) peersFingerprint() string {
	pairs := make([]string, 0, len(inst.Peers))
	for _, p := range inst.Peers {
		pairs = append(pairs, fmt.Sprintf(
			"%s=%s;psk=%s;ips=%s;ka=%d;q=%d;exp=%d",
			p.Name, p.PublicKey, p.PresharedKey, strings.Join(p.AllowedIPs, ","), p.KeepAlive, p.QuotaBytes, p.ExpiresUnix,
		))
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

// awg2ClientSettings mirrors the shape the panel already stores WireGuard
// clients in (model.Client's wg-related fields), so the same client editor
// UI/API payload works for both protocols with no bespoke schema.
type awg2ClientSettings struct {
	Email        string   `json:"email"`
	PublicKey    string   `json:"publicKey"`
	PreSharedKey string   `json:"preSharedKey"`
	AllowedIPs   []string `json:"allowedIPs"`
	KeepAlive    int      `json:"keepAlive"`
	Enable       bool     `json:"enable"`
	TotalGB      int64    `json:"totalGB"`
	ExpiryTime   int64    `json:"expiryTime"`
}

// awg2InboundSettings is the top-level shape of an awg2 inbound's Settings
// JSON: one [Interface] worth of scalars plus a "clients" array in the same
// place and shape every other protocol uses (unlike the WireGuard inbound,
// which stores "peers"; awg2 never round-trips through Xray so there is no
// peers/clients duality to reconcile).
type awg2InboundSettings struct {
	Backend    string   `json:"backend"`
	PrivateKey string   `json:"privateKey"`
	Address    []string `json:"address"`
	MTU        int      `json:"mtu"`

	Jc, Jmin, Jmax int
	S1, S2, S3, S4 int
	H1, H2, H3, H4 uint32
	I1, I2, I3, I4, I5 string

	Clients []awg2ClientSettings `json:"clients"`
}

// UnmarshalJSON is defined explicitly (instead of relying on struct tags for
// the obfuscation fields) only to keep the field block above readable; the
// JSON keys are the lowercase parameter names AmneziaWG documents (jc, jmin,
// jmax, s1..s4, h1..h4, i1..i5), matching bivlked/amneziawg-installer and the
// upstream AmneziaWG client apps so exported .conf files and imported ones
// use identical key casing.
func (s *awg2InboundSettings) UnmarshalJSON(data []byte) error {
	type alias awg2InboundSettings
	aux := struct {
		Jc   int    `json:"jc"`
		Jmin int    `json:"jmin"`
		Jmax int    `json:"jmax"`
		S1   int    `json:"s1"`
		S2   int    `json:"s2"`
		S3   int    `json:"s3"`
		S4   int    `json:"s4"`
		H1   uint32 `json:"h1"`
		H2   uint32 `json:"h2"`
		H3   uint32 `json:"h3"`
		H4   uint32 `json:"h4"`
		I1   string `json:"i1"`
		I2   string `json:"i2"`
		I3   string `json:"i3"`
		I4   string `json:"i4"`
		I5   string `json:"i5"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.Jc, s.Jmin, s.Jmax = aux.Jc, aux.Jmin, aux.Jmax
	s.S1, s.S2, s.S3, s.S4 = aux.S1, aux.S2, aux.S3, aux.S4
	s.H1, s.H2, s.H3, s.H4 = aux.H1, aux.H2, aux.H3, aux.H4
	s.I1, s.I2, s.I3, s.I4, s.I5 = aux.I1, aux.I2, aux.I3, aux.I4, aux.I5
	return nil
}

// InstanceFromInbound derives a desired Instance from an awg2 inbound. Returns
// false when the inbound is not a usable awg2 inbound (wrong protocol,
// unparseable settings, or missing the server private key without which no
// interface can be brought up).
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.AmneziaWG2 {
		return Instance{}, false
	}
	var parsed awg2InboundSettings
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		return Instance{}, false
	}
	if strings.TrimSpace(parsed.PrivateKey) == "" || len(parsed.Address) == 0 {
		return Instance{}, false
	}

	backend := Backend(strings.TrimSpace(parsed.Backend))
	if backend != BackendUserspace {
		backend = BackendKernel
	}

	peers := make([]Peer, 0, len(parsed.Clients))
	for _, c := range parsed.Clients {
		if !c.Enable || c.PublicKey == "" || c.Email == "" {
			continue
		}
		p := Peer{
			Name:         c.Email,
			PublicKey:    c.PublicKey,
			PresharedKey: c.PreSharedKey,
			AllowedIPs:   c.AllowedIPs,
			KeepAlive:    c.KeepAlive,
		}
		if c.TotalGB > 0 {
			p.QuotaBytes = c.TotalGB
		}
		if c.ExpiryTime > 0 {
			p.ExpiresUnix = c.ExpiryTime / 1000
		}
		if len(p.AllowedIPs) == 0 {
			// A peer with no AllowedIPs is legal WireGuard syntax but
			// accepts no traffic, which almost always means the client
			// record predates IP assignment; skip it rather than emit a
			// half-configured peer that silently never routes anything.
			continue
		}
		peers = append(peers, p)
	}

	mtu := parsed.MTU
	if mtu <= 0 {
		mtu = 1380 // AmneziaWG's own default; lower than plain WG's 1420 to
		// leave room for the obfuscation header/junk overhead.
	}

	return Instance{
		Id:         ib.Id,
		Tag:        ib.Tag,
		Backend:    backend,
		PrivateKey: strings.TrimSpace(parsed.PrivateKey),
		Address:    parsed.Address,
		ListenPort: ib.Port,
		MTU:        mtu,
		Jc:         parsed.Jc, Jmin: parsed.Jmin, Jmax: parsed.Jmax,
		S1: parsed.S1, S2: parsed.S2, S3: parsed.S3, S4: parsed.S4,
		H1: parsed.H1, H2: parsed.H2, H3: parsed.H3, H4: parsed.H4,
		I1: parsed.I1, I2: parsed.I2, I3: parsed.I3, I4: parsed.I4, I5: parsed.I5,
		Peers: peers,
	}, true
}

// renderConfig builds the awg-quick-compatible configuration file for an
// instance: one [Interface] section carrying the obfuscation parameters,
// followed by one [Peer] section per active client. This is intentionally
// the same file format the AmneziaWG/AmneziaVPN client apps read, so the
// same rendering (minus the server-only PrivateKey/ListenPort/Address, plus
// the peer's own PrivateKey and the server's PublicKey as its own peer) is
// reused for the downloadable client .conf in the QR/export path.
func renderConfig(inst Instance) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	if len(inst.Address) > 0 {
		fmt.Fprintf(&b, "Address = %s\n", strings.Join(inst.Address, ", "))
	}
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.ListenPort)
	if inst.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", inst.MTU)
	}
	writeIntParam(&b, "Jc", inst.Jc)
	writeIntParam(&b, "Jmin", inst.Jmin)
	writeIntParam(&b, "Jmax", inst.Jmax)
	writeIntParam(&b, "S1", inst.S1)
	writeIntParam(&b, "S2", inst.S2)
	writeIntParam(&b, "S3", inst.S3)
	writeIntParam(&b, "S4", inst.S4)
	writeUintParam(&b, "H1", inst.H1)
	writeUintParam(&b, "H2", inst.H2)
	writeUintParam(&b, "H3", inst.H3)
	writeUintParam(&b, "H4", inst.H4)
	writeStrParam(&b, "I1", inst.I1)
	writeStrParam(&b, "I2", inst.I2)
	writeStrParam(&b, "I3", inst.I3)
	writeStrParam(&b, "I4", inst.I4)
	writeStrParam(&b, "I5", inst.I5)

	for _, p := range inst.Peers {
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "# %s\n", p.Name)
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
		if len(p.AllowedIPs) > 0 {
			fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
		}
		if p.KeepAlive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.KeepAlive)
		}
	}
	return b.String()
}

func writeIntParam(b *strings.Builder, key string, v int) {
	if v != 0 {
		fmt.Fprintf(b, "%s = %d\n", key, v)
	}
}

func writeUintParam(b *strings.Builder, key string, v uint32) {
	if v != 0 {
		fmt.Fprintf(b, "%s = %d\n", key, v)
	}
}

func writeStrParam(b *strings.Builder, key, v string) {
	if v != "" {
		fmt.Fprintf(b, "%s = %s\n", key, v)
	}
}
