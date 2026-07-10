package amneziawg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id:       7,
		Tag:      "inbound-7",
		Port:     51820,
		Protocol: model.AmneziaWG2,
		Settings: `{
			"privateKey":"cHJpdmF0ZWtleXByaXZhdGVrZXlwcml2YXRla2V5eHg=",
			"address":["10.20.30.1/24","fd00:awg2::1/64"],
			"mtu":1380,
			"backend":"kernel",
			"jc":4,"jmin":40,"jmax":70,
			"s1":0,"s2":0,"s3":0,"s4":0,
			"h1":1234567,"h2":2345678,"h3":3456789,"h4":4567890,
			"clients":[
				{"email":"alice","publicKey":"YWxpY2VwdWJrZXlhbGljZXB1YmtleWFsaWNlcHVi","allowedIPs":["10.20.30.2/32"],"enable":true,"totalGB":1073741824,"expiryTime":1893456000000},
				{"email":"bob","publicKey":"","allowedIPs":["10.20.30.3/32"],"enable":true},
				{"email":"carol","publicKey":"Y2Fyb2xwdWJrZXljYXJvbHB1YmtleWNhcm9scHVi","allowedIPs":[],"enable":true},
				{"email":"dave","publicKey":"ZGF2ZXB1YmtleWRhdmVwdWJrZXlkYXZlcHVi","allowedIPs":["10.20.30.5/32"],"enable":false}
			]
		}`,
	}

	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Id != 7 || inst.ListenPort != 51820 {
		t.Fatalf("bad instance identity: %+v", inst)
	}
	if inst.Backend != BackendKernel {
		t.Fatalf("expected kernel backend, got %q", inst.Backend)
	}
	if len(inst.Address) != 2 || inst.Address[0] != "10.20.30.1/24" {
		t.Fatalf("address pool not parsed: %+v", inst.Address)
	}
	if inst.Jc != 4 || inst.Jmin != 40 || inst.Jmax != 70 {
		t.Fatalf("obfuscation junk params not parsed: %+v", inst)
	}
	if inst.H1 != 1234567 || inst.H4 != 4567890 {
		t.Fatalf("header magic params not parsed: %+v", inst)
	}

	// Only alice qualifies: bob has no public key, carol has no AllowedIPs,
	// dave is disabled.
	if len(inst.Peers) != 1 {
		t.Fatalf("expected exactly one usable peer, got %d: %+v", len(inst.Peers), inst.Peers)
	}
	if inst.Peers[0].Name != "alice" {
		t.Fatalf("unexpected peer: %+v", inst.Peers[0])
	}
	if inst.Peers[0].QuotaBytes != 1073741824 {
		t.Fatalf("totalGB must map to the byte quota, got %d", inst.Peers[0].QuotaBytes)
	}
	if inst.Peers[0].ExpiresUnix != 1893456000 {
		t.Fatalf("expiryTime (ms) must map to a unix-second deadline, got %d", inst.Peers[0].ExpiresUnix)
	}

	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.WireGuard}); ok {
		t.Fatal("non-awg2 inbound should not produce an instance")
	}
	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.AmneziaWG2, Settings: `{"address":["10.0.0.1/24"]}`}); ok {
		t.Fatal("an inbound with no server private key should not produce an instance")
	}
	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.AmneziaWG2, Settings: `{"privateKey":"x"}`}); ok {
		t.Fatal("an inbound with no address pool should not produce an instance")
	}
}

func TestInstanceFromInboundUserspaceFallback(t *testing.T) {
	ib := &model.Inbound{
		Id: 1, Port: 51821, Protocol: model.AmneziaWG2,
		Settings: `{"privateKey":"a","address":["10.0.0.1/24"],"backend":"userspace"}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok || inst.Backend != BackendUserspace {
		t.Fatalf("expected userspace backend to be honored, got %+v ok=%v", inst, ok)
	}

	// Anything other than the two known backend strings falls back to kernel,
	// which keeps a typo (or an older settings blob predating the field)
	// from silently leaving the instance backend-less.
	ib2 := &model.Inbound{
		Id: 2, Port: 51822, Protocol: model.AmneziaWG2,
		Settings: `{"privateKey":"a","address":["10.0.0.1/24"],"backend":"bogus"}`,
	}
	inst2, ok := InstanceFromInbound(ib2)
	if !ok || inst2.Backend != BackendKernel {
		t.Fatalf("expected fallback to kernel backend, got %+v ok=%v", inst2, ok)
	}
}

func TestRenderConfig(t *testing.T) {
	bare := renderConfig(Instance{
		PrivateKey: "SERVERKEY", Address: []string{"10.0.0.1/24"}, ListenPort: 51820,
	})
	for _, unwanted := range []string{"Jc", "Jmin", "Jmax", "S1", "H1", "MTU", "[Peer]"} {
		if strings.Contains(bare, unwanted) {
			t.Fatalf("bare config should not contain %q:\n%s", unwanted, bare)
		}
	}
	if !strings.Contains(bare, "PrivateKey = SERVERKEY\n") {
		t.Fatalf("missing PrivateKey:\n%s", bare)
	}
	if !strings.Contains(bare, "Address = 10.0.0.1/24\n") {
		t.Fatalf("missing Address:\n%s", bare)
	}
	if !strings.Contains(bare, "ListenPort = 51820\n") {
		t.Fatalf("ListenPort must always be present:\n%s", bare)
	}

	full := renderConfig(Instance{
		PrivateKey: "SERVERKEY",
		Address:    []string{"10.0.0.1/24", "fd00::1/64"},
		ListenPort: 443, MTU: 1380,
		Jc: 4, Jmin: 40, Jmax: 70,
		S1: 10, S2: 20, S3: 0, S4: 0,
		H1: 1111, H2: 2222, H3: 3333, H4: 4444,
		I1: "<b 0xAABBCCDD>",
		Peers: []Peer{
			{Name: "alice", PublicKey: "PUBALICE", PresharedKey: "PSKALICE", AllowedIPs: []string{"10.0.0.2/32"}, KeepAlive: 25},
			{Name: "bob", PublicKey: "PUBBOB", AllowedIPs: []string{"10.0.0.3/32"}},
		},
	})
	for _, want := range []string{
		"Address = 10.0.0.1/24, fd00::1/64\n",
		"MTU = 1380\n",
		"Jc = 4\n", "Jmin = 40\n", "Jmax = 70\n",
		"S1 = 10\n", "S2 = 20\n",
		"H1 = 1111\n", "H4 = 4444\n",
		"I1 = <b 0xAABBCCDD>\n",
		"[Peer]\n# alice\n",
		"PublicKey = PUBALICE\n",
		"PresharedKey = PSKALICE\n",
		"AllowedIPs = 10.0.0.2/32\n",
		"PersistentKeepalive = 25\n",
		"[Peer]\n# bob\n",
		"PublicKey = PUBBOB\n",
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("full config missing %q:\n%s", want, full)
		}
	}
	if strings.Contains(full, "S3 = ") || strings.Contains(full, "S4 = ") {
		t.Fatalf("zero-valued obfuscation params must be omitted:\n%s", full)
	}
	if strings.Contains(full, "PresharedKey = \n") {
		t.Fatalf("a peer without a PSK must omit the key entirely, not emit it blank:\n%s", full)
	}
	// [Interface] must precede every [Peer], and PublicKey must always be
	// present for a rendered peer even without a keepalive/PSK.
	if strings.Index(full, "[Interface]") > strings.Index(full, "[Peer]") {
		t.Fatalf("[Interface] must precede [Peer] sections:\n%s", full)
	}
}

func TestFingerprintSplit(t *testing.T) {
	base := Instance{
		PrivateKey: "a", Address: []string{"10.0.0.1/24"}, ListenPort: 51820,
		Peers: []Peer{{Name: "alice", PublicKey: "pub-a", AllowedIPs: []string{"10.0.0.2/32"}}},
	}

	structuralMutations := map[string]func(*Instance){
		"backend":    func(i *Instance) { i.Backend = BackendUserspace },
		"privateKey": func(i *Instance) { i.PrivateKey = "b" },
		"address":    func(i *Instance) { i.Address = []string{"10.0.0.1/24", "fd00::1/64"} },
		"port":       func(i *Instance) { i.ListenPort = 443 },
		"mtu":        func(i *Instance) { i.MTU = 1420 },
		"jc":         func(i *Instance) { i.Jc = 4 },
		"jmin":       func(i *Instance) { i.Jmin = 40 },
		"jmax":       func(i *Instance) { i.Jmax = 70 },
		"s1":         func(i *Instance) { i.S1 = 1 },
		"h1":         func(i *Instance) { i.H1 = 999 },
		"i1":         func(i *Instance) { i.I1 = "<b 0x01>" },
	}
	for name, mutate := range structuralMutations {
		t.Run("structural/"+name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if base.structuralFingerprint() == changed.structuralFingerprint() {
				t.Fatalf("structural fingerprint must change when %s changes", name)
			}
			if base.peersFingerprint() != changed.peersFingerprint() {
				t.Fatalf("peers fingerprint must stay put when %s changes", name)
			}
		})
	}

	peerMutations := map[string]func(*Instance){
		"add":    func(i *Instance) { i.Peers = append(i.Peers, Peer{Name: "bob", PublicKey: "pub-b"}) },
		"rekey":  func(i *Instance) { i.Peers = []Peer{{Name: "alice", PublicKey: "pub-a2"}} },
		"remove": func(i *Instance) { i.Peers = nil },
		"rename": func(i *Instance) { i.Peers = []Peer{{Name: "alice2", PublicKey: "pub-a"}} },
		"psk":    func(i *Instance) { i.Peers = []Peer{{Name: "alice", PublicKey: "pub-a", PresharedKey: "psk"}} },
		"quota":  func(i *Instance) { i.Peers = []Peer{{Name: "alice", PublicKey: "pub-a", QuotaBytes: 1 << 30}} },
	}
	for name, mutate := range peerMutations {
		t.Run("peers/"+name, func(t *testing.T) {
			changed := base
			changed.Peers = append([]Peer(nil), base.Peers...)
			mutate(&changed)
			if base.peersFingerprint() == changed.peersFingerprint() {
				t.Fatalf("peers fingerprint must change on a %s", name)
			}
			if base.structuralFingerprint() != changed.structuralFingerprint() {
				t.Fatalf("structural fingerprint must stay put on a %s", name)
			}
		})
	}

	t.Run("orderInsensitive", func(t *testing.T) {
		forward := Instance{Peers: []Peer{{Name: "alice", PublicKey: "a"}, {Name: "bob", PublicKey: "b"}}}
		reversed := Instance{Peers: []Peer{{Name: "bob", PublicKey: "b"}, {Name: "alice", PublicKey: "a"}}}
		if forward.peersFingerprint() != reversed.peersFingerprint() {
			t.Fatal("peers fingerprint must not depend on peer order")
		}
	})
}

func TestIfaceNameFitsLinuxLimit(t *testing.T) {
	// IFNAMSIZ is 16 bytes including the NUL terminator, i.e. 15 usable
	// characters. A four-digit inbound id is a realistic upper bound.
	inst := Instance{Id: 9999}
	if got := inst.IfaceName(); len(got) > 15 {
		t.Fatalf("interface name %q (%d bytes) exceeds the 15-byte Linux limit", got, len(got))
	}
}
