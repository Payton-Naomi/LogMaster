package serial

import (
	"fmt"
	"sort"
	"strings"
)

// MatchPort selects by the strongest configured stable identity. A criterion
// producing multiple candidates is ambiguous and never falls back to a weaker one.
func MatchPort(want PortMatch, ports []PortDescriptor) (PortDescriptor, error) {
	type criterion struct {
		configured bool
		matches    func(PortDescriptor) bool
	}
	criteria := []criterion{
		{
			configured: strings.TrimSpace(want.USBSerial) != "",
			matches: func(port PortDescriptor) bool {
				return equalIdentity(port.USBSerial, want.USBSerial)
			},
		},
		{
			configured: strings.TrimSpace(want.VID) != "" && strings.TrimSpace(want.PID) != "" && strings.TrimSpace(want.Location) != "",
			matches: func(port PortDescriptor) bool {
				return equalIdentity(port.VID, want.VID) &&
					equalIdentity(port.PID, want.PID) &&
					equalLocation(port.Location, want.Location)
			},
		},
		{
			configured: strings.TrimSpace(want.PortName) != "",
			matches: func(port PortDescriptor) bool {
				return strings.EqualFold(strings.TrimSpace(port.Name), strings.TrimSpace(want.PortName))
			},
		},
	}

	configured := false
	for _, criterion := range criteria {
		if !criterion.configured {
			continue
		}
		configured = true
		var candidates []PortDescriptor
		for _, port := range ports {
			if criterion.matches(port) {
				candidates = append(candidates, port)
			}
		}
		switch len(candidates) {
		case 0:
			continue
		case 1:
			return candidates[0], nil
		default:
			names := make([]string, len(candidates))
			for i := range candidates {
				names[i] = candidates[i].Name
			}
			sort.Strings(names)
			return PortDescriptor{}, fmt.Errorf("%w: %s", ErrAmbiguousPort, strings.Join(names, ", "))
		}
	}
	if !configured {
		return PortDescriptor{}, fmt.Errorf("%w: no stable identity or explicit port configured", ErrPortNotFound)
	}
	return PortDescriptor{}, ErrPortNotFound
}

func equalIdentity(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func equalLocation(a, b string) bool {
	normalize := func(value string) string {
		return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	}
	return normalize(a) == normalize(b)
}
