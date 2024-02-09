// Copyright 2019-2024 The Inspektor Gadget authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !withoutebpf

package tracer

import (
	"context"
	"fmt"
	"net/netip"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"

	gadgetcontext "github.com/inspektor-gadget/inspektor-gadget/pkg/gadget-context"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/internal/networktracer"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/logger"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -cc clang -cflags ${CFLAGS} -type event_t dns ./bpf/dns.c -- $CLANG_OS_FLAGS -I./bpf/

const (
	BPFQueryMapName = "query_map"
	MaxAddrAnswers  = 8 // Keep aligned with MAX_ADDR_ANSWERS in bpf/dns-common.h
)

// needs to be kept in sync with dnsEventT from dns_bpfel.go (without the Anaddr field)
type dnsEventTAbbrev struct {
	Netns       uint32
	_           [4]byte
	Timestamp   uint64
	MountNsId   uint64
	Pid         uint32
	Tid         uint32
	Uid         uint32
	Gid         uint32
	Task        [16]uint8
	SaddrV6     [16]uint8
	DaddrV6     [16]uint8
	Af          uint16
	Sport       uint16
	Dport       uint16
	Proto       uint8
	_           [1]byte
	Id          uint16
	Qtype       uint16
	Qr          uint8
	PktType     uint8
	Rcode       uint8
	_           [1]byte
	LatencyNs   uint64
	Name        [255]uint8
	_           [1]byte
	Ancount     uint16
	Anaddrcount uint16
}

type Tracer struct {
	*networktracer.Tracer[types.Event]

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTracer() (*Tracer, error) {
	t := &Tracer{}

	if err := t.install(); err != nil {
		t.Close()
		return nil, fmt.Errorf("installing tracer: %w", err)
	}

	// timeout not configurable in this case
	if err := t.run(context.TODO(), log.StandardLogger(), time.Minute); err != nil {
		t.Close()
		return nil, fmt.Errorf("running tracer: %w", err)
	}

	return t, nil
}

// pkt_type definitions:
// https://github.com/torvalds/linux/blob/v5.14-rc7/include/uapi/linux/if_packet.h#L26
var pktTypeNames = []string{
	"HOST",
	"BROADCAST",
	"MULTICAST",
	"OTHERHOST",
	"OUTGOING",
	"LOOPBACK",
	"USER",
	"KERNEL",
}

// List taken from:
// https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-4
var qTypeNames = map[uint]string{
	1:     "A",
	2:     "NS",
	3:     "MD",
	4:     "MF",
	5:     "CNAME",
	6:     "SOA",
	7:     "MB",
	8:     "MG",
	9:     "MR",
	10:    "NULL",
	11:    "WKS",
	12:    "PTR",
	13:    "HINFO",
	14:    "MINFO",
	15:    "MX",
	16:    "TXT",
	17:    "RP",
	18:    "AFSDB",
	19:    "X25",
	20:    "ISDN",
	21:    "RT",
	22:    "NSAP",
	23:    "NSAP-PTR",
	24:    "SIG",
	25:    "KEY",
	26:    "PX",
	27:    "GPOS",
	28:    "AAAA",
	29:    "LOC",
	30:    "NXT",
	31:    "EID",
	32:    "NIMLOC",
	33:    "SRV",
	34:    "ATMA",
	35:    "NAPTR",
	36:    "KX",
	37:    "CERT",
	38:    "A6",
	39:    "DNAME",
	40:    "SINK",
	41:    "OPT",
	42:    "APL",
	43:    "DS",
	44:    "SSHFP",
	45:    "IPSECKEY",
	46:    "RRSIG",
	47:    "NSEC",
	48:    "DNSKEY",
	49:    "DHCID",
	50:    "NSEC3",
	51:    "NSEC3PARAM",
	52:    "TLSA",
	53:    "SMIMEA",
	55:    "HIP",
	56:    "NINFO",
	57:    "RKEY",
	58:    "TALINK",
	59:    "CDS",
	60:    "CDNSKEY",
	61:    "OPENPGPKEY",
	62:    "CSYNC",
	63:    "ZONEMD",
	64:    "SVCB",
	65:    "HTTPS",
	99:    "SPF",
	100:   "UINFO",
	101:   "UID",
	102:   "GID",
	103:   "UNSPEC",
	104:   "NID",
	105:   "L32",
	106:   "L64",
	107:   "LP",
	108:   "EUI48",
	109:   "EUI64",
	249:   "TKEY",
	250:   "TSIG",
	251:   "IXFR",
	252:   "AXFR",
	253:   "MAILB",
	254:   "MAILA",
	255:   "*",
	256:   "URI",
	257:   "CAA",
	258:   "AVC",
	259:   "DOA",
	260:   "AMTRELAY",
	32768: "TA",
	32769: "DLV",
}

const MaxDNSName = int(unsafe.Sizeof(dnsEventT{}.Name))

// DNS header RCODE (response code) field.
// https://datatracker.ietf.org/doc/rfc1035#section-4.1.1
var rCodeNames = map[uint8]string{
	0: "NoError",
	1: "FormErr",
	2: "ServFail",
	3: "NXDomain",
	4: "NotImp",
	5: "Refused",
}

// parseLabelSequence parses a label sequence into a string with dots.
// See https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.2
func parseLabelSequence(sample []byte) (ret string) {
	sampleBounded := make([]byte, MaxDNSName)
	copy(sampleBounded, sample)

	for i := 0; i < MaxDNSName; i++ {
		length := int(sampleBounded[i])
		if length == 0 {
			break
		}
		if i+1+length < MaxDNSName {
			ret += string(sampleBounded[i+1:i+1+length]) + "."
		}
		i += length
	}
	return ret
}

func bpfEventToDNSEvent(bpfEvent *dnsEventTAbbrev, answers []byte, netns uint64) (*types.Event, error) {
	event := types.Event{
		Event: eventtypes.Event{
			Type: eventtypes.NORMAL,
		},
		Pid:           bpfEvent.Pid,
		Tid:           bpfEvent.Tid,
		Uid:           bpfEvent.Uid,
		Gid:           bpfEvent.Gid,
		WithMountNsID: eventtypes.WithMountNsID{MountNsID: bpfEvent.MountNsId},
		WithNetNsID:   eventtypes.WithNetNsID{NetNsID: netns},
		Comm:          gadgets.FromCString(bpfEvent.Task[:]),
	}
	event.Event.Timestamp = gadgets.WallTimeFromBootTime(bpfEvent.Timestamp)

	event.ID = fmt.Sprintf("%.4x", bpfEvent.Id)
	ipversion := gadgets.IPVerFromAF(bpfEvent.Af)
	event.SrcIP = gadgets.IPStringFromBytes(bpfEvent.SaddrV6, ipversion)
	event.DstIP = gadgets.IPStringFromBytes(bpfEvent.DaddrV6, ipversion)

	if bpfEvent.Qr == 1 {
		event.Qr = types.DNSPktTypeResponse
		event.Nameserver = event.SrcIP
	} else {
		event.Qr = types.DNSPktTypeQuery
		event.Nameserver = event.DstIP
	}

	event.SrcPort = bpfEvent.Sport
	event.DstPort = bpfEvent.Dport
	event.Protocol = gadgets.ProtoString(int(bpfEvent.Proto))

	// Convert name into a string with dots
	event.DNSName = parseLabelSequence(bpfEvent.Name[:])

	// Parse the packet type
	event.PktType = "UNKNOWN"
	pktTypeUint := uint(bpfEvent.PktType)
	if pktTypeUint < uint(len(pktTypeNames)) {
		event.PktType = pktTypeNames[pktTypeUint]
	}

	qTypeUint := uint(bpfEvent.Qtype)
	var ok bool
	event.QType, ok = qTypeNames[qTypeUint]
	if !ok {
		event.QType = "UNASSIGNED"
	}

	if bpfEvent.Qr == 1 {
		rCodeUint := uint8(bpfEvent.Rcode)
		event.Rcode, ok = rCodeNames[rCodeUint]
		if !ok {
			event.Rcode = "UNKNOWN"
		}

		event.Latency = time.Duration(bpfEvent.LatencyNs)
	}

	// There's a limit on the number of addresses in the BPF event,
	// so bpfEvent.AnaddrCount is always less than or equal to bpfEvent.Ancount
	event.NumAnswers = int(bpfEvent.Ancount)
	for i := uint16(0); i < bpfEvent.Anaddrcount; i++ {
		// For A records, the address in the bpf event will be
		// IPv4-mapped-IPv6, which netip.Addr.Unmap() converts back to IPv4.
		addr := netip.AddrFrom16([16]byte(answers[i*16 : i*16+16])).Unmap().String()
		event.Addresses = append(event.Addresses, addr)
	}

	return &event, nil
}

// --- Registry changes

func (g *GadgetDesc) NewInstance() (gadgets.Gadget, error) {
	return &Tracer{}, nil
}

func (t *Tracer) Init(gadgetCtx gadgets.GadgetContext) error {
	if err := t.install(); err != nil {
		t.Close()
		return fmt.Errorf("installing tracer: %w", err)
	}

	t.ctx, t.cancel = gadgetcontext.WithTimeoutOrCancel(gadgetCtx.Context(), gadgetCtx.Timeout())

	return nil
}

func (t *Tracer) install() error {
	networkTracer, err := networktracer.NewTracer[types.Event]()
	if err != nil {
		return fmt.Errorf("creating network tracer: %w", err)
	}
	t.Tracer = networkTracer
	return nil
}

func (t *Tracer) run(ctx context.Context, logger logger.Logger, dnsTimeout time.Duration) error {
	spec, err := loadDns()
	if err != nil {
		return fmt.Errorf("loading asset: %w", err)
	}

	parseDNSEvent := func(rawSample []byte, netns uint64) (*types.Event, error) {
		bpfEvent := (*dnsEventTAbbrev)(unsafe.Pointer(&rawSample[0]))
		// 4 (padding) + (size of event without addresses) + (address count * 16)
		expected := 4 + int(unsafe.Sizeof(*bpfEvent)) + int(bpfEvent.Anaddrcount)*16
		if len(rawSample) != expected {
			return nil, fmt.Errorf("invalid sample size: received: %d vs expected: %d",
				len(rawSample), expected)
		}

		event, err := bpfEventToDNSEvent(bpfEvent, rawSample[unsafe.Offsetof(dnsEventT{}.Anaddr):], netns)
		if err != nil {
			return nil, err
		}

		return event, nil
	}

	if err := t.Tracer.Run(spec, types.Base, parseDNSEvent); err != nil {
		return fmt.Errorf("setting network tracer spec: %w", err)
	}

	// Start a background thread to garbage collect queries without responses
	// from the queries map (used to calculate DNS latency).
	// The goroutine terminates when t.ctx is done.
	queryMap := t.Tracer.GetMap(BPFQueryMapName)
	if queryMap == nil {
		t.Close()
		return fmt.Errorf("got nil retrieving DNS query map")
	}
	startGarbageCollector(ctx, logger, dnsTimeout, queryMap)

	return nil
}

func (t *Tracer) Run(gadgetCtx gadgets.GadgetContext) error {
	dnsTimeout := gadgetCtx.GadgetParams().Get(ParamDNSTimeout).AsDuration()

	if err := t.run(t.ctx, gadgetCtx.Logger(), dnsTimeout); err != nil {
		return err
	}

	<-t.ctx.Done()
	return nil
}

func (t *Tracer) Close() {
	if t.cancel != nil {
		t.cancel()
	}

	if t.Tracer != nil {
		t.Tracer.Close()
	}
}
