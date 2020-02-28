package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"net"
	"os"
)

func main() {
	var ipv4 bool
	var ipv6 bool

	flag.CommandLine.SetOutput(os.Stdout)
	file := flag.String("db", "", "MaxmindDB file with the IP ranges for countries")
	flag.BoolVar(&ipv4, "ipv4", false, "Prints IPV4 addresses")
	flag.BoolVar(&ipv6,"ipv6", false, "Prints IPV6 addresses")
	flag.Usage = usage
	flag.Parse()
	
	// If no IP version is specified, we default to IPV4.
	if ipv4 == false && ipv6 == false {
		ipv4 = true
	}
	if len(flag.Args()) == 0 {
		check(errors.New("need to specify at least one ISO country code"))
	}
	db, err := maxminddb.Open(*file)
	check(err)
	defer db.Close()

	record := geoip2.Country{}

	networks := db.Networks()
	for networks.Next() {
		subnet, err := networks.Network(&record)
		check(err)
		address := subnet.IP
		if isZeros(address) == false {
			// Long story short: this should be good enough for my case
			if address.To4() != nil {
				if ipv4==true {
					fmt.Println(subnet.String())
				}
			} else {
				if ipv6==true {
					fmt.Println(subnet.String())
				}
			}
		}
	}
	check(networks.Err())

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

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "%s [-ipv4] [-ipv6] {-db option_argument} [country_code...]\n", os.Args[0])
	fmt.Fprintln(flag.CommandLine.Output(), "Prints a list of networks for the countries specified as ISO 3166-1 alpha2 code")
	flag.PrintDefaults()
}