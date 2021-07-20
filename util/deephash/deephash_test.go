// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deephash

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"inet.af/netaddr"
	"tailscale.com/tailcfg"
	"tailscale.com/types/ipproto"
	"tailscale.com/util/dnsname"
	"tailscale.com/version"
	"tailscale.com/wgengine/filter"
	"tailscale.com/wgengine/router"
	"tailscale.com/wgengine/wgcfg"
)

func TestHash(t *testing.T) {
	type tuple [2]interface{}
	tests := []struct {
		in     tuple
		wantEq bool
	}{
		{in: tuple{false, true}, wantEq: false},
		{in: tuple{true, true}, wantEq: true},
		{in: tuple{false, false}, wantEq: true},
		{
			in: func() tuple {
				i1 := 1
				i2 := 2
				v1 := [3]*int{&i1, &i2, &i1}
				v2 := [3]*int{&i1, &i2, &i2}
				return tuple{v1, v2}
			}(),
			wantEq: false,
		},
	}

	for _, tt := range tests {
		gotEq := Hash(tt.in[0]) == Hash(tt.in[1])
		if gotEq != tt.wantEq {
			t.Errorf("(Hash(%v) == Hash(%v)) = %v, want %v", tt.in[0], tt.in[1], gotEq, tt.wantEq)
		}
	}
}

func TestDeepHash(t *testing.T) {
	// v contains the types of values we care about for our current callers.
	// Mostly we're just testing that we don't panic on handled types.
	v := getVal()

	hash1 := Hash(v)
	t.Logf("hash: %v", hash1)
	for i := 0; i < 20; i++ {
		hash2 := Hash(getVal())
		if hash1 != hash2 {
			t.Error("second hash didn't match")
		}
	}
}

func getVal() []interface{} {
	return []interface{}{
		&wgcfg.Config{
			Name:      "foo",
			Addresses: []netaddr.IPPrefix{netaddr.IPPrefixFrom(netaddr.IPFrom16([16]byte{3: 3}), 5)},
			Peers: []wgcfg.Peer{
				{
					Endpoints: wgcfg.Endpoints{
						IPPorts: wgcfg.NewIPPortSet(netaddr.MustParseIPPort("42.42.42.42:5")),
					},
				},
			},
		},
		&router.Config{
			Routes: []netaddr.IPPrefix{
				netaddr.MustParseIPPrefix("1.2.3.0/24"),
				netaddr.MustParseIPPrefix("1234::/64"),
			},
		},
		map[dnsname.FQDN][]netaddr.IP{
			dnsname.FQDN("a."): {netaddr.MustParseIP("1.2.3.4"), netaddr.MustParseIP("4.3.2.1")},
			dnsname.FQDN("b."): {netaddr.MustParseIP("8.8.8.8"), netaddr.MustParseIP("9.9.9.9")},
			dnsname.FQDN("c."): {netaddr.MustParseIP("6.6.6.6"), netaddr.MustParseIP("7.7.7.7")},
			dnsname.FQDN("d."): {netaddr.MustParseIP("6.7.6.6"), netaddr.MustParseIP("7.7.7.8")},
			dnsname.FQDN("e."): {netaddr.MustParseIP("6.8.6.6"), netaddr.MustParseIP("7.7.7.9")},
			dnsname.FQDN("f."): {netaddr.MustParseIP("6.9.6.6"), netaddr.MustParseIP("7.7.7.0")},
		},
		map[dnsname.FQDN][]netaddr.IPPort{
			dnsname.FQDN("a."): {netaddr.MustParseIPPort("1.2.3.4:11"), netaddr.MustParseIPPort("4.3.2.1:22")},
			dnsname.FQDN("b."): {netaddr.MustParseIPPort("8.8.8.8:11"), netaddr.MustParseIPPort("9.9.9.9:22")},
			dnsname.FQDN("c."): {netaddr.MustParseIPPort("8.8.8.8:12"), netaddr.MustParseIPPort("9.9.9.9:23")},
			dnsname.FQDN("d."): {netaddr.MustParseIPPort("8.8.8.8:13"), netaddr.MustParseIPPort("9.9.9.9:24")},
			dnsname.FQDN("e."): {netaddr.MustParseIPPort("8.8.8.8:14"), netaddr.MustParseIPPort("9.9.9.9:25")},
		},
		map[tailcfg.DiscoKey]bool{
			{1: 1}: true,
			{1: 2}: false,
			{2: 3}: true,
			{3: 4}: false,
		},
		&tailcfg.MapResponse{
			DERPMap: &tailcfg.DERPMap{
				Regions: map[int]*tailcfg.DERPRegion{
					1: &tailcfg.DERPRegion{
						RegionID:   1,
						RegionCode: "foo",
						Nodes: []*tailcfg.DERPNode{
							{
								Name:     "n1",
								RegionID: 1,
								HostName: "foo.com",
							},
							{
								Name:     "n2",
								RegionID: 1,
								HostName: "bar.com",
							},
						},
					},
				},
			},
			DNSConfig: &tailcfg.DNSConfig{
				Resolvers: []tailcfg.DNSResolver{
					{Addr: "10.0.0.1"},
				},
			},
			PacketFilter: []tailcfg.FilterRule{
				{
					SrcIPs: []string{"1.2.3.4"},
					DstPorts: []tailcfg.NetPortRange{
						{
							IP:    "1.2.3.4/32",
							Ports: tailcfg.PortRange{First: 1, Last: 2},
						},
					},
				},
			},
			Peers: []*tailcfg.Node{
				{
					ID: 1,
				},
				{
					ID: 2,
				},
			},
			UserProfiles: []tailcfg.UserProfile{
				{ID: 1, LoginName: "foo@bar.com"},
				{ID: 2, LoginName: "bar@foo.com"},
			},
		},
		filter.Match{
			IPProto: []ipproto.Proto{1, 2, 3},
		},
	}
}

var sink = Hash("foo")

func BenchmarkHash(b *testing.B) {
	b.ReportAllocs()
	v := getVal()
	for i := 0; i < b.N; i++ {
		sink = Hash(v)
	}
}

func TestHashMapAcyclic(t *testing.T) {
	m := map[int]string{}
	for i := 0; i < 100; i++ {
		m[i] = fmt.Sprint(i)
	}
	got := map[string]bool{}

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	for i := 0; i < 20; i++ {
		v := reflect.ValueOf(m)
		buf.Reset()
		bw.Reset(&buf)
		h := &hasher{
			bw:         bw,
			visitStack: map[uintptr]int{},
		}
		h.hashMap(v)
		if got[string(buf.Bytes())] {
			continue
		}
		got[string(buf.Bytes())] = true
	}
	if len(got) != 1 {
		t.Errorf("got %d results; want 1", len(got))
	}
}

func TestPrintArray(t *testing.T) {
	type T struct {
		X [32]byte
	}
	x := T{X: [32]byte{1: 1, 31: 31}}
	var got bytes.Buffer
	bw := bufio.NewWriter(&got)
	h := &hasher{
		bw:         bw,
		visitStack: map[uintptr]int{},
	}
	h.print(reflect.ValueOf(x))
	bw.Flush()
	const want = "struct" +
		"\x00\x00\x00\x00\x00\x00\x00\x01" + // 1 field
		"\x00\x00\x00\x00\x00\x00\x00\x00" + // 0th field
		// the 32 bytes:
		"\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x1f"
	if got := got.Bytes(); string(got) != want {
		t.Errorf("wrong:\n got: %q\nwant: %q\n", got, want)
	}
}

func BenchmarkHashMapAcyclic(b *testing.B) {
	b.ReportAllocs()
	m := map[int]string{}
	for i := 0; i < 100; i++ {
		m[i] = fmt.Sprint(i)
	}

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	v := reflect.ValueOf(m)

	h := &hasher{
		bw:         bw,
		visitStack: map[uintptr]int{},
	}

	for i := 0; i < b.N; i++ {
		buf.Reset()
		bw.Reset(&buf)
		h.hashMap(v)
	}
}

func BenchmarkTailcfgNode(b *testing.B) {
	b.ReportAllocs()

	node := new(tailcfg.Node)
	for i := 0; i < b.N; i++ {
		sink = Hash(node)
	}
}

func TestExhaustive(t *testing.T) {
	seen := make(map[Sum]bool)
	for i := 0; i < 100000; i++ {
		s := Hash(i)
		if seen[s] {
			t.Fatalf("hash collision %v", i)
		}
		seen[s] = true
	}
}

// verify this doesn't loop forever, as it used to (Issue 2340)
func TestMapCyclicFallback(t *testing.T) {
	type T struct {
		M map[string]interface{}
	}
	v := &T{
		M: map[string]interface{}{},
	}
	v.M["m"] = v.M
	Hash(v)
}

func TestArrayAllocs(t *testing.T) {
	if version.IsRace() {
		t.Skip("skipping test under race detector")
	}
	type T struct {
		X [32]byte
	}
	x := &T{X: [32]byte{1: 1, 2: 2, 3: 3, 4: 4}}
	n := int(testing.AllocsPerRun(1000, func() {
		sink = Hash(x)
	}))
	if n > 0 {
		t.Errorf("allocs = %v; want 0", n)
	}
}

func BenchmarkHashArray(b *testing.B) {
	b.ReportAllocs()
	type T struct {
		X [32]byte
	}
	x := &T{X: [32]byte{1: 1, 2: 2, 3: 3, 4: 4}}

	for i := 0; i < b.N; i++ {
		sink = Hash(x)
	}
}
