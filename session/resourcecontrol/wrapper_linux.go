//go:build linux

package resourcecontrol

import (
	"fmt"
	"strings"
)

// ioniceClassNum maps ionice_class string values to ionice(1) numeric class codes.
var ioniceClassNum = map[string]int{
	"none":        1,
	"best-effort": 2,
	"idle":        3,
}

// platformShellPrefix returns a shell-safe nice/ionice prefix string on Linux.
func (w *Wrapper) platformShellPrefix() string {
	p := w.policy
	_, niceErr := w.lookPath("nice")
	niceStr := ""
	if niceErr == nil {
		niceStr = fmt.Sprintf("nice -n %d", p.Nice)
	}

	if p.IoniceClass == "" || p.IoniceClass == "none" {
		if niceErr != nil {
			w.emitWarn("nice not found in PATH; running command without resource wrapper")
			return ""
		}
		return niceStr
	}

	if _, err := w.lookPath("ionice"); err != nil {
		if niceErr != nil {
			w.emitWarn("nice not found in PATH; running command without resource wrapper")
			return ""
		}
		w.emitWarn("ionice not found in PATH; falling back to nice only (install util-linux for I/O scheduling)")
		return niceStr
	}

	classNum := ioniceClassNum[p.IoniceClass]
	parts := make([]string, 0, 2)
	if niceStr != "" {
		parts = append(parts, niceStr)
	} else {
		w.emitWarn("nice not found in PATH; falling back to ionice only")
	}
	parts = append(parts, fmt.Sprintf("ionice -c %d", classNum))
	if p.IoniceClass == "best-effort" && p.IoniceLevel > 0 {
		parts = append(parts, fmt.Sprintf("-n %d", p.IoniceLevel))
	}
	return strings.Join(parts, " ")
}

// platformWrapExec returns (ionice-path, [ionice-args..., nice, -n, N, name, args...])
// when both helpers are available; falls back to nice-only; falls back to original
// command when neither is found.
func (w *Wrapper) platformWrapExec(name string, args []string) (string, []string) {
	p := w.policy

	ionicePath, ioniceErr := w.lookPath("ionice")
	nicePath, niceErr := w.lookPath("nice")

	useIonice := ioniceErr == nil && p.IoniceClass != "" && p.IoniceClass != "none"

	if useIonice {
		classNum := ioniceClassNum[p.IoniceClass]
		ioniceArgs := []string{"-c", fmt.Sprintf("%d", classNum)}
		if p.IoniceClass == "best-effort" && p.IoniceLevel > 0 {
			ioniceArgs = append(ioniceArgs, "-n", fmt.Sprintf("%d", p.IoniceLevel))
		}
		var finalArgs []string
		if niceErr == nil {
			// ionice <class-args> nice -n <N> <cmd> <args...>
			finalArgs = append(ioniceArgs, nicePath, "-n", fmt.Sprintf("%d", p.Nice), name)
		} else {
			// ionice <class-args> <cmd> <args...>
			finalArgs = append(ioniceArgs, name)
		}
		finalArgs = append(finalArgs, args...)
		return ionicePath, finalArgs
	}

	if ioniceErr != nil && p.IoniceClass != "" && p.IoniceClass != "none" {
		w.emitWarn("ionice not found in PATH; falling back to nice only (install util-linux for I/O scheduling)")
	}

	if niceErr != nil {
		w.emitWarn("nice not found in PATH; running command without resource wrapper")
		return name, args
	}

	// nice -n <N> <cmd> <args...>
	newArgs := []string{"-n", fmt.Sprintf("%d", p.Nice), name}
	newArgs = append(newArgs, args...)
	return nicePath, newArgs
}
