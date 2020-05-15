package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const size = net.IPv4len

// Address is, basically, an IPv4 address as int.
// I am being lazy here, as is much simpler to handle intervals this way
// than to figure out a general way to do it with byte slices (and for IPv6 too).
type Address struct {
	IntValue  uint32 // 4 bytes
	IPAddress []byte // This is not really necessary, but handy
}

// The general problem with using CIDR and masks for IP networks intervals,
// is that they don't always match and you might end up with multiple CIDR
// for describing a single interval.
// This is, exactly, what is happening with the GeoIP databases. Where you
// get multiple /22 subnets, for example, to describe a single interval.
// That's fine for lookups, but what I want is to reduce the number of elements
// in the set.
// Also, as far as I understand it, intervals are the way subnets are entered in nftables.
// Lower address is included, upper is not [,).
type Interval struct {
	lower *Address
	upper *Address
}

func (n *Interval) Lower() net.IP {
	return n.lower.ToIP()
}

func (n *Interval) Upper() net.IP {
	return n.upper.ToIP()
}

// New Interval initialise a network interval from a CIDR string.
func NewInterval(CIDR string) *Interval {
	/*
		I found that using the CIDR string make it much easier to convert it to
		an interval.
		Again, a lazy solution, that, maybe will be fixed when I try to generalise
		this.
	*/
	ip, subnet, err := net.ParseCIDR(CIDR)
	if err != nil {
		return nil
	}
	if bytes.Equal(subnet.Mask[len(subnet.Mask)-size:], []byte{255, 255, 255, 255}) {
		return nil
	}
	if ip == nil || isZeros(ip) {
		return nil
	}
	network := NewAddress(ip)
	if network == nil {
		return nil
	}
	if !network.Valid() {
		return nil
	}
	broadcastAddress := broadcast(*subnet)
	if !broadcastAddress.Valid() {
		return nil
	}
	nextAddress := broadcastAddress.Next()
	if !nextAddress.Valid() {
		return nil
	}
	n := &Interval{}
	n.lower = network
	n.upper = nextAddress
	return n
}

// Valid evaluate an address according to various criteria that make it
// not suitable to be used in an interval. Not necessarily a bad IP address.
func (a Address) Valid() bool {
	if len(a.IPAddress) != size {
		return false
	}
	if isZeros(a.IPAddress) {
		return false
	}
	if a.IntValue == 0 {
		return false
	}
	return true
}

func (a Address) String() string {
	return a.ToIP().String()
}

func NewAddress(ipv4 net.IP) *Address {
	ip := ipv4.To4()
	if ip == nil {
		return nil
	}
	a := &Address{
		IPAddress: ip,
		IntValue:  a2i(ip),
	}
	return a
}

func a2i(ipv4 net.IP) uint32 {
	var i uint32
	ip := ipv4.To4()
	if ip == nil || len(ip) != net.IPv4len {
		log.Fatalf("invalid IPv4 address: %s", ipv4.String())
	}
	buf := bytes.NewReader(ip)
	err := binary.Read(buf, binary.BigEndian, &i)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func i2IP(i uint32) net.IP {
	var buf = new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, i)
	if err != nil {
		log.Fatal(err)
	}
	ip := buf.Bytes()
	if len(ip) > size {
		log.Fatalf("error converting %d to bytes", i)
	}
	ipv4 := net.IPv4(
		ip[0],
		ip[1],
		ip[2],
		ip[3],
	)
	return ipv4
}

func netMask(subnet net.IPMask) net.IPMask {
	m := subnet[len(subnet)-size:]
	return net.IPv4Mask(
		m[0], m[1], m[2], m[3],
	)
}

// The bits in the hostmask for IPv4 are the ones NOT in the netmask.
func hostMask(netMask net.IPMask) net.IPMask {
	return net.IPv4Mask(
		^netMask[0],
		^netMask[1],
		^netMask[2],
		^netMask[3],
	)
}

func broadcast(subNet net.IPNet) *Address {
	h := hostMask(netMask(subNet.Mask))
	ip := subNet.IP
	broadcast := net.IPv4(
		ip[0]|h[0],
		ip[1]|h[1],
		ip[2]|h[2],
		ip[3]|h[3],
	)
	return NewAddress(broadcast)
}

func (a Address) Next() *Address {
	if !a.Valid() {
		return nil
	}
	nextIP := i2IP(a.IntValue + 1)
	return NewAddress(nextIP)
}

func (a Address) ToIP() net.IP {
	return net.IPv4(
		a.IPAddress[0],
		a.IPAddress[1],
		a.IPAddress[2],
		a.IPAddress[3],
	)
}

/*
func (n *Interval) Contains(ip net.IP) bool {

	for _,subNet := range n.subNets {
		if subNet.Contains(ip) {
			return true
		}
	}
	return false
}
*/

func CanJoin(a *Interval, b *Interval) bool {
	aLower := a.lower.IntValue
	bLower := b.lower.IntValue
	aUpper := a.upper.IntValue
	bUpper := b.upper.IntValue
	// a overlaps lower end of b
	if aLower <= bLower && aUpper >= bLower {
		return true
	}
	// a overlaps upper end of b
	if aLower <= bUpper && aLower >= bLower {
		return true
	}
	return false
}

func Join(a *Interval, b *Interval) *Interval {
	if !CanJoin(a, b) {
		return nil
	}
	n := &Interval{}
	n.lower = min(
		a.lower,
		b.lower,
	)
	n.upper = max(
		a.upper,
		b.upper,
	)
	return n
}

func min(a *Address, b *Address) *Address {
	aValue := a.IntValue
	bValue := b.IntValue
	if aValue <= bValue {
		return a
	}
	return b
}

func max(a *Address, b *Address) *Address {
	aValue := a.IntValue
	bValue := b.IntValue
	if aValue >= bValue {
		return a
	}
	return b
}

/*
func mergeSubNets(a []net.IPNet, b []net.IPNet) []net.IPNet {
	var subNets = make([]net.IPNet, len(a))
	copy(subNets, a)
	for _,i := range b {
		dup := false
		for _,j := range a {
			// Shortcut
			if i.String() == j.String() {
				dup = true
				break
			}
		}
		if ! dup {
			subNets = append(subNets, i)
		}
	}
	return subNets
}
*/

func (n *Interval) String() string {
	return fmt.Sprintf(
		"%s - %s",
		n.Lower(), n.Upper(),
	)
}

// Copied from net package
// Is p all zeros?
func isZeros(p net.IP) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != 0 {
			return false
		}
	}
	return true
}

/*
func (n Interval)SubNets() []string {
	subNets := make([]string, len(n.subNets))
	for i,subnet := range n.subNets {
		subNets[i] = subnet.String()
	}
	return subNets
}

*/
