package payload

import (
	"errors"
	"slices"
)

const marker = "__payload_namespace"

const mountFields = 4

// An empty Type binds Source onto Target.
type Mount struct {
	Source string
	Target string
	Type   string
	Data   string
}

func namespaceArgs(mounts []Mount, path string, args []string) []string {
	out := []string{marker}
	for _, m := range mounts {
		out = append(out, m.Source, m.Target, m.Type, m.Data)
	}
	out = append(out, "--", path)
	return append(out, args...)
}

func parseNamespace(args []string) ([]Mount, []string, error) {
	sep := slices.Index(args, "--")
	if sep < 0 || sep%mountFields != 0 || sep == len(args)-1 {
		return nil, nil, errors.New("payload: malformed namespace arguments")
	}
	mounts := make([]Mount, 0, sep/mountFields)
	for i := 0; i < sep; i += mountFields {
		mounts = append(
			mounts,
			Mount{Source: args[i], Target: args[i+1], Type: args[i+2], Data: args[i+3]},
		)
	}
	return mounts, args[sep+1:], nil
}
