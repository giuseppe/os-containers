// +build selinux

package oscontainers

import (
	"os"
	"syscall"
	"unsafe"

	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

// #cgo pkg-config: libselinux
// #include <stdlib.h>
// #include <selinux/selinux.h>
// #include <selinux/label.h>
import "C"

type SELinuxCtx struct {
	hnd *C.struct_selabel_handle
}

func makeSELinuxCtx() (*SELinuxCtx, error) {
	if os.Getuid() == 0 && selinux.GetEnabled() {
		selinuxHnd, err := C.selabel_open(C.SELABEL_CTX_FILE, nil, 0)
		if selinuxHnd == nil {
			return nil, errors.Wrapf(err, "cannot open the SELinux DB")
		}
		return &SELinuxCtx{hnd: selinuxHnd}, nil
	}
	return &SELinuxCtx{hnd: nil}, nil
}

func (ctx *SELinuxCtx) Close() error {
	if ctx.hnd != nil {
		C.selabel_close(ctx.hnd)
	}
	return nil
}

func (ctx *SELinuxCtx) Label(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))
	var context *C.char
	res, err := C.selabel_lookup_raw(ctx.hnd, &context, pathC, C.int(info.Mode()&os.ModePerm))
	if int(res) < 0 && err != syscall.ENOENT {
		return errors.Wrapf(err, "cannot selabel_lookup_raw %s", path)
	}

	if int(res) == 0 {
		defer C.freecon(context)
		res, err = C.lsetfilecon_raw(pathC, context)
		if int(res) < 0 {
			return errors.Wrapf(err, "cannot setfilecon_raw %s", pathC)
		}
	}
	return nil
}
