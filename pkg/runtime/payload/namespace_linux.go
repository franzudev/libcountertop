package payload

import (
	"fmt"
	"os"
	"syscall"
)

func Enter() {
	if len(os.Args) < 2 || os.Args[1] != marker {
		return
	}
	if err := isolate(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(127)
	}
}

func isolate(args []string) error {
	mounts, argv, err := parseNamespace(args)
	if err != nil {
		return err
	}
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("payload: private mounts: %w", err)
	}
	for _, m := range mounts {
		if m.Type == "" {
			if err := syscall.Mount(m.Source, m.Target, "", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("payload: bind %s on %s: %w", m.Source, m.Target, err)
			}
			continue
		}
		flags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV)
		if err := syscall.Mount(m.Source, m.Target, m.Type, flags, m.Data); err != nil {
			return fmt.Errorf("payload: %s on %s: %w", m.Type, m.Target, err)
		}
	}
	return syscall.Exec(argv[0], argv, os.Environ())
}

func isolated(c Command) (string, []string, *syscall.SysProcAttr, error) {
	a := attrs()
	a.Unshareflags = syscall.CLONE_NEWNS
	return "/proc/self/exe", namespaceArgs(c.Mounts, c.Path, c.Args), a, nil
}
