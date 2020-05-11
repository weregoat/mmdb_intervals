package main

import (
	"net"
	"testing"
)

func TestNewAddress(t *testing.T) {
	tests := []struct {
		IP          string
		ExpectedInt uint64
	}{
		{"0.0.0.1", 1},
		{"300.0.0.1", 0},
		{"255.255.255.255", 4294967295},
		{"0.0.1.0", 256 * 1},
		{"0.1.0.0", 256 * 256 * 1},
		{"1.0.0.0", 256 * 256 * 256 * 1},
		{"0.1.1.0", 256*256*1 + 256*1},
		{"10.0.0.22", 256*256*256*10 + 22},
		{"0.1.0.0", 256 * 256 * 1},
		{"192.168.0.1", 256*256*256*192 + 256*256*168 + 1},
		{"223.255.255.0", 256*256*256*223 + 256*256*255 + 256*255},
	}
	for _, test := range tests {
		eip := net.ParseIP(test.IP)
		a := NewAddress(eip)
		if eip == nil {
			if a != nil {
				t.Logf("Expecting no address from bad IP %s, got %v", test.IP, *a)
			}
		} else {
			i := a.IntValue.Uint64()
			ei := test.ExpectedInt

			ip := a.ToIP()
			t.Logf("IP %s => Address %s(uint: %d)", test.IP, ip, i)
			if i != ei {
				t.Logf("expecting int value for %s to be %d, got %d", test.IP, ei, i)
				t.Fail()
			}
			if !ip.Equal(eip) {
				t.Logf("expecting IP to be %s, got %s", eip.String(), ip.String())
			}
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		CIDR    string
		Address string
		Next    string
	}{
		{"0.0.1.0/24", "0.0.1.0", "0.0.2.0"},
		{"192.160.0.0/12", "192.160.0.0", "192.176.0.0"},
		{"10.0.0.0/8", "10.0.0.0", "11.0.0.0"},
		{"223.252.192.0/18", "223.252.192.0", "223.253.0.0"},
	}
	for _, test := range tests {
		ip, subnet, err := net.ParseCIDR(test.CIDR)
		t.Logf("%s", subnet.String())
		n := NewInterval(test.CIDR)
		if err != nil {
			if n != nil {
				t.Logf("Expecting no network from bad CIDR %s, got %v", test.CIDR, n)
			}
		} else {
			if !ip.Equal(subnet.IP) {
				t.Errorf("expecting subnet IP of CIDR %s to be %s, got %s", test.CIDR, ip, subnet.IP)
			}
			t.Logf(
				"CIDR %s => %s",
				test.CIDR,
				n.String(),
			)
			eNext := net.ParseIP(test.Next).To4()
			eIP := net.ParseIP(test.Address).To4()
			if !n.lower.ToIP().Equal(eIP) {
				t.Logf(
					"expecting network IP for CIDR %s to be %s, got %s",
					test.CIDR,
					eIP.String(),
					n.Lower().String(),
				)
				t.Fail()
			}
			if !n.upper.ToIP().Equal(eNext) {
				t.Logf(
					"expecting next network IP for CIDR %s to be %s, got %s",
					test.CIDR,
					eNext.String(),
					n.upper.String(),
				)
				t.Fail()
			}

		}
		// Test host address (/32)
		CIDR := "192.168.11.12/32"
		n = NewInterval(CIDR)
		if n != nil {
			t.Logf("Expecting no network from /32 CIDR, but got %v", n)
		}
	}
}

func TestCanJoin(t *testing.T) {
	tests := []struct {
		A       string
		B       string
		CanJoin bool
	}{
		{"0.0.1.0/24", "0.0.1.0/24", true},              // Same subnet
		{"192.160.0.0/12", "192.176.0.0/24", true},      // Adjacent, different mask
		{"192.176.0.0/24", "192.160.0.0/12", true},      // Adjacent, different mask - inverted
		{"10.0.0.0/8", "10.0.0.0/16", true},             // Inclusion
		{"42.0.0.0/24", "42.0.0.0/16", true},            // Inclusion - inverted
		{"223.252.192.0/24", "223.252.194.0/24", false}, // A < B
		{"10.0.0.0/8", "1.0.0.0/16", false},             // B < A
	}
	for _, test := range tests {
		a := NewInterval(test.A)
		b := NewInterval(test.B)
		if a != nil && b != nil {
			result := CanJoin(a, b)
			if result != test.CanJoin {
				t.Logf("Expected check on join between %s(%s) and %s(%s) to be %t, but got %t",
					a.String(),
					test.A,
					b.String(),
					test.B,
					test.CanJoin,
					result,
				)
				t.Fail()
			}
		}
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		A     string
		B     string
		Lower string
		Upper string
	}{
		{"0.0.1.0/24", "0.0.1.0/24", "0.0.1.0", "0.0.2.0"},                 // Same subnet
		{"192.160.0.0/12", "192.176.0.0/24", "192.160.0.0", "192.176.1.0"}, // Adjacent, different mask
		{"192.176.0.0/24", "192.160.0.0/12", "192.160.0.0", "192.176.1.0"}, // Adjacent, different mask - inverted
		{"10.0.0.0/8", "10.0.0.0/16", "10.0.0.0", "11.0.0.0"},              // Inclusion
		{"42.1.0.0/24", "42.1.0.0/16", "42.1.0.0", "42.2.0.0"},             // Inclusion - inverted
	}
	for _, test := range tests {
		a := NewInterval(test.A)
		b := NewInterval(test.B)
		if a != nil && b != nil {
			result := Join(a, b)
			if result.Lower().String() != test.Lower ||
				result.Upper().String() != test.Upper {
				t.Logf("Expecting join between join between %s(%s) and %s(%s) to be %s - %s, but got %s",
					a.String(),
					test.A,
					b.String(),
					test.B,
					test.Lower,
					test.Upper,
					result.String(),
				)
				t.Fail()
			}
		}
	}
}
