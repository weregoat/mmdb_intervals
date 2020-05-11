package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/nftables"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"log"

	"os"
	"strings"
)

const batchSize = 1000

var debug bool

func main() {

	flag.CommandLine.SetOutput(os.Stdout)
	file := flag.String("db", "", "MaxmindDB file with the IP ranges for countries")
	list := flag.Bool("print", false, "Prints resulting networks")
	setName := flag.String("set", "", "Add networks to nftables set")
	tableName := flag.String("table", "filter", "Name of the nftable the set is in")
	flag.BoolVar(&debug, "debug", false, "Print debug logs (very verbose)")
	flag.Usage = usage
	flag.Parse()

	countries := flag.Args()
	if len(countries) == 0 {
		check(errors.New("need to specify at least one ISO country code"))
	}
	db, err := maxminddb.Open(*file)
	check(err)
	defer db.Close()

	record := geoip2.Country{}

	var intervals []*Interval
	networks := db.Networks()
	for networks.Next() {
		subnet, err := networks.Network(&record)
		check(err)
		// Only IPv4, for now
		if subnet.IP.To4() != nil {

			for _, country := range countries {
				if record.Country.IsoCode == country {
					dLog(
						fmt.Sprintf("subnet %s assigned to %s",
							subnet.String(),
							record.Country.IsoCode,
						),
					)
					new := NewInterval(subnet.String())
					if new != nil {
						add := true
						intLen := len(intervals)
						for i := 0; i < intLen; i++ {
							present := intervals[i]
							if CanJoin(new, present) {
								intervals[i] = Join(new, present)
								dLog(
									fmt.Sprintf(
										"subnet %s merged into %s",
										subnet.String(),
										intervals[i].String(),
									),
								)
								add = false
								break
							}
						}
						if add {
							intervals = append(intervals, new)
						}
					}
				}
			}
		}
	}

	check(networks.Err())
	if len(*setName) > 0 && len(*tableName) > 0 {
		err = addToSet(*tableName, *setName, intervals)
		check(err)

	}
	if *list {
		print(intervals)
	}

}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func dLog(message string) {
	if debug {
		log.Print(message)
	}
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "%s [-print] [-set nft_set] [-table nft_table] {-db option_argument} [country_code...]\n", os.Args[0])
	fmt.Fprintln(flag.CommandLine.Output(), "Reads the network intervals for the countries specified as ISO 3166-1 alpha2 code from a MaxmindDB GeoIP2 database")
	flag.PrintDefaults()
}

func addToSet(tableName, setName string, intervals []*Interval) error {
	var set *nftables.Set
	conn := &nftables.Conn{}
	tables, err := conn.ListTables()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if strings.EqualFold(table.Name, tableName) {
			set, err = conn.GetSetByName(table, setName)
			if err != nil {
				return err
			}
			break
		}
	}
	if set == nil {
		return fmt.Errorf(
			"could not find a set named %+q in table %+q",
			setName, tableName,
		)
	}

	var elements []nftables.SetElement
	for _, interval := range intervals {
		if interval != nil {
			start := nftables.SetElement{
				Key: interval.Lower().To4(),
			}
			end := nftables.SetElement{
				Key:         interval.Upper().To4(),
				IntervalEnd: true,
			}
			elements = append(elements, start, end)
		}
	}

	for start := 0; start < len(elements); start += batchSize {
		end := start + batchSize
		if end > len(elements) {
			end = len(elements)
		}
		dLog(
			fmt.Sprintf(
				"Adding elements from %d to %d to @%s",
				start,
				end,
				set.Name,
			),
		)
		err = conn.SetAddElements(set, elements[start:end])
		if err != nil {
			return err
		}
		err = conn.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func print(intervals []*Interval) {
	for _, interval := range intervals {
		fmt.Println(interval.String())
	}
}
