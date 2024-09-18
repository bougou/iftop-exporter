package iftop

import "fmt"

type Options struct {
	InterfaceName        string
	NoHostnameLookup     bool
	NoPortConvert        bool
	ShowPort             bool
	SortBy               SortBy
	ShowBandwidthInBytes bool
	NumberOfLines        int
	SingleSeconds        int
	useTextMode          bool
}

type SortBy string

const (
	SortBy2s          SortBy = "2s"
	SortBy10s         SortBy = "10s"
	SortBy40s         SortBy = "40s"
	SortBySource      SortBy = "source"
	SortByDestination SortBy = "destination"
)

func (options *Options) Valid() (err error) {
	if options.InterfaceName == "" {
		return fmt.Errorf("interface name is required")
	}

	return nil
}

func getArguments(options Options) []string {
	arguments := []string{}

	if options.InterfaceName != "" {
		arguments = append(arguments, "-i", options.InterfaceName)
	}

	if options.NoHostnameLookup {
		arguments = append(arguments, "-n")
	}

	if options.NoPortConvert {
		arguments = append(arguments, "-N")
	}

	if options.ShowPort {
		arguments = append(arguments, "-P")
	}

	if options.SortBy != "" {
		arguments = append(arguments, "-o", string(options.SortBy))
	}

	if options.ShowBandwidthInBytes {
		arguments = append(arguments, "-B")
	}

	if options.useTextMode {
		arguments = append(arguments, "-t")
	}

	if options.NumberOfLines != 0 {
		arguments = append(arguments, "-L", fmt.Sprintf("%d", options.NumberOfLines))
	}

	if options.SingleSeconds != 0 {
		arguments = append(arguments, "-s", fmt.Sprintf("%d", options.SingleSeconds))
	}

	return arguments

}
