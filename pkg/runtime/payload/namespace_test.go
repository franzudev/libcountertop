package payload

import (
	"slices"
	"testing"
)

func TestNamespaceArgumentsRoundTrip(t *testing.T) {
	mounts := []Mount{
		{Source: "/data/etc", Target: "/system/etc"},
		{Source: "tmpfs", Target: "/data/state/tmp", Type: "tmpfs", Data: "size=192m,mode=0700"},
		{Source: "/a", Target: "/b"},
	}
	args := namespaceArgs(
		mounts,
		"/engine/chromium.sh",
		[]string{"--remote-debugging-port=9222", "--"},
	)
	if args[0] != marker {
		t.Fatalf("first argument %q, want the marker", args[0])
	}
	got, argv, err := parseNamespace(args[1:])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, mounts) {
		t.Fatalf("mounts %v, want %v", got, mounts)
	}
	want := []string{"/engine/chromium.sh", "--remote-debugging-port=9222", "--"}
	if !slices.Equal(argv, want) {
		t.Fatalf("command %v, want %v", argv, want)
	}
}

func TestNamespaceArgumentsRejectMalformedInput(t *testing.T) {
	for _, args := range [][]string{
		{"/only-source", "--", "/bin/true"},
		{"/a", "/b", "--", "/bin/true"},
		{"/a", "/b", "", "", "--"},
		{"/a", "/b", "", "", "/bin/true"},
	} {
		if _, _, err := parseNamespace(args); err == nil {
			t.Fatalf("parseNamespace(%q) accepted malformed input", args)
		}
	}
}
