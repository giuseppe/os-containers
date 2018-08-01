// +build !selinux

package oscontainers

import (
	"os"

	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

// #cgo pkg-config: libselinux
// #include <selinux/selinux.h>
// #include <selinux/label.h>
import "C"

type SELinuxCtx struct {
}

func makeSELinuxCtx() (*SelinuxCtx, error) {
	return &SELinuxCtx{}, nil
}

func (ctx *SELinuxCtx) Close() error {
	return nil
}

func (ctx *SELinuxCtx) Label(path string) error {
	return nil
}
