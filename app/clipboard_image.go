package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

var (
	readClipboardText            = clipboard.ReadAll
	captureClipboardImage        = captureClipboardImageToTempFile
	clipboardImageLookPath       = exec.LookPath
	clipboardImageCommand        = exec.CommandContext
	errClipboardImageNotFound    = errors.New("clipboard does not contain an image")
	errClipboardImageNoHelpers   = errors.New("clipboard image paste requires pngpaste, wl-paste, or xclip")
	errClipboardImageUnsupported = errors.New("clipboard image paste is not supported on this platform")
)

type clipboardImageCaptureSpec struct {
	name    string
	capture func(context.Context) (string, error)
}

func captureClipboardImageToTempFile(ctx context.Context) (string, error) {
	specs := clipboardImageCaptureSpecs()
	if len(specs) == 0 {
		return "", errClipboardImageUnsupported
	}

	var lastErr error
	available := false
	for _, spec := range specs {
		if _, err := clipboardImageLookPath(spec.name); err != nil {
			continue
		}
		available = true
		path, err := spec.capture(ctx)
		if err == nil {
			return path, nil
		}
		lastErr = err
	}
	if !available {
		return "", errClipboardImageNoHelpers
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errClipboardImageNotFound
}

func clipboardImageCaptureSpecs() []clipboardImageCaptureSpec {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardImageCaptureSpec{{
			name: "pngpaste",
			capture: func(ctx context.Context) (string, error) {
				return captureClipboardImageViaFileArg(ctx, "pngpaste")
			},
		}}
	case "linux":
		return []clipboardImageCaptureSpec{
			{
				name: "wl-paste",
				capture: func(ctx context.Context) (string, error) {
					return captureClipboardImageViaStdout(ctx, "wl-paste", "--no-newline", "--type", "image/png")
				},
			},
			{
				name: "xclip",
				capture: func(ctx context.Context) (string, error) {
					return captureClipboardImageViaStdout(ctx, "xclip", "-selection", "clipboard", "-t", "image/png", "-o")
				},
			},
		}
	default:
		return nil
	}
}

func captureClipboardImageViaFileArg(ctx context.Context, name string, args ...string) (string, error) {
	path, cleanup, err := newClipboardImageTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		if path != "" {
			cleanup()
		}
	}()

	cmdArgs := append(args, path)
	cmd := clipboardImageCommand(ctx, name, cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", clipboardCaptureError(err, stderr.String())
	}
	if empty, err := clipboardImageFileEmpty(path); err != nil {
		return "", err
	} else if empty {
		return "", errClipboardImageNotFound
	}

	out := path
	path = ""
	return out, nil
}

func captureClipboardImageViaStdout(ctx context.Context, name string, args ...string) (string, error) {
	path, cleanup, err := newClipboardImageTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		if path != "" {
			cleanup()
		}
	}()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()

	cmd := clipboardImageCommand(ctx, name, args...)
	cmd.Stdout = file
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", clipboardCaptureError(err, stderr.String())
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if empty, err := clipboardImageFileEmpty(path); err != nil {
		return "", err
	} else if empty {
		return "", errClipboardImageNotFound
	}

	out := path
	path = ""
	return out, nil
}

func newClipboardImageTempFile() (string, func(), error) {
	file, err := os.CreateTemp("", "kasmos-clipboard-*.png")
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, func() {
		_ = os.Remove(path)
	}, nil
}

func clipboardImageFileEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Size() == 0, nil
}

func clipboardCaptureError(err error, stderr string) error {
	if stderr != "" {
		return fmt.Errorf("%w: %s", errClipboardImageNotFound, strings.TrimSpace(stderr))
	}
	if err != nil {
		return fmt.Errorf("%w: %v", errClipboardImageNotFound, err)
	}
	return errClipboardImageNotFound
}
