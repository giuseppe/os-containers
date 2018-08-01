package oscontainers

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"unsafe"

	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
)

// #cgo pkg-config: glib-2.0 gobject-2.0 ostree-1
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <stdlib.h>
// #include <ostree.h>
// #include <gio/ginputstream.h>
// static OstreeRepoCheckoutAtOptions *MakeOSTreeCheckoutOptions() {
//   OstreeRepoCheckoutAtOptions *r = malloc (sizeof (*r));
//   if (r == NULL)
//     return r;
//   memset (r, 0, sizeof (*r));
//   r->mode = geteuid () != 0 ? OSTREE_REPO_CHECKOUT_MODE_USER : OSTREE_REPO_CHECKOUT_MODE_NONE;
//   r->overwrite_mode = OSTREE_REPO_CHECKOUT_OVERWRITE_UNION_FILES;
//   return r;
// }
import "C"

var ostreePrefix = "ociimage"

type OSTreeRepo struct {
	repo *C.struct_OstreeRepo
}

/* already live in containers/image.  */
var ostreeRefRegexp = regexp.MustCompile(`^[A-Za-z0-9.-]$`)

func decodeOStreeRef(in string) string {
	var buffer bytes.Buffer
	i := 0
	for {
		if i == len(in) {
			break
		}
		if in[i] == '_' {
			b, err := strconv.ParseInt(in[i+1:i+3], 16, 8)
			if err != nil {
				continue
			}
			buffer.WriteByte(byte(b))
			i = i + 3

		} else {
			buffer.WriteByte(in[i])
			i = i + 1
		}
	}
	return buffer.String()
}

func encodeOStreeRef(in string) string {
	var buffer bytes.Buffer
	for i := range in {
		sub := in[i : i+1]
		if ostreeRefRegexp.MatchString(sub) {
			buffer.WriteString(sub)
		} else {
			buffer.WriteString(fmt.Sprintf("_%02X", sub[0]))
		}
	}
	return buffer.String()
}

func openRepo(path string) (*OSTreeRepo, error) {
	var cerr *C.GError
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	repo := C.ostree_repo_new(pathc)
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_open(repo, nil, &cerr)))
	if !r {
		C.g_object_unref(C.gpointer(repo))
		return nil, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return &OSTreeRepo{repo: repo}, nil
}

func (repo *OSTreeRepo) hasBranch(commit string) (bool, error) {
	var cerr *C.GError
	var ref *C.char
	defer C.free(unsafe.Pointer(ref))

	cCommit := C.CString(commit)
	defer C.free(unsafe.Pointer(cCommit))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.repo, cCommit, C.gboolean(1), &ref, &cerr))) {
		return false, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return ref != nil, nil
}

func (repo *OSTreeRepo) readMetadata(commit, key string) (bool, string, error) {
	var cerr *C.GError
	var ref *C.char
	defer C.free(unsafe.Pointer(ref))

	cCommit := C.CString(commit)
	defer C.free(unsafe.Pointer(cCommit))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.repo, cCommit, C.gboolean(1), &ref, &cerr))) {
		return false, "", glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}

	if ref == nil {
		return false, "", nil
	}

	var variant *C.GVariant
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_load_variant(repo.repo, C.OSTREE_OBJECT_TYPE_COMMIT, ref, &variant, &cerr))) {
		return false, "", glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	defer C.g_variant_unref(variant)
	if variant != nil {
		cKey := C.CString(key)
		defer C.free(unsafe.Pointer(cKey))

		metadata := C.g_variant_get_child_value(variant, 0)
		defer C.g_variant_unref(metadata)

		data := C.g_variant_lookup_value(metadata, (*C.gchar)(cKey), nil)
		if data != nil {
			defer C.g_variant_unref(data)
			ptr := (*C.char)(C.g_variant_get_string(data, nil))
			val := C.GoString(ptr)
			return true, val, nil
		}
	}
	return false, "", nil
}

func (repo *OSTreeRepo) unionCheckout(layer string, dirfd int, dest string) error {
	var cerr *C.GError
	var ref *C.char
	defer C.free(unsafe.Pointer(ref))

	cBranch := C.CString(fmt.Sprintf("%s/%s", ostreePrefix, layer))
	defer C.free(unsafe.Pointer(cBranch))

	cDest := C.CString(dest)
	defer C.free(unsafe.Pointer(cDest))

	options := C.MakeOSTreeCheckoutOptions()
	if options == nil {
		return fmt.Errorf("cannot allocate checkout options")
	}
	defer C.free(unsafe.Pointer(options))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.repo, cBranch, C.gboolean(1), &ref, &cerr))) {
		return glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_checkout_at(repo.repo, options, C.int(dirfd), cDest, ref, nil, &cerr))) {
		return glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return nil
}

func (repo *OSTreeRepo) getBranches(prefix string) (map[string]string, error) {
	var cerr *C.GError

	cPrefix := C.CString(prefix)
	defer C.free(unsafe.Pointer(cPrefix))

	var h *C.GHashTable

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_list_refs(repo.repo, cPrefix, &h, nil, &cerr))) {
		return nil, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	defer C.g_hash_table_unref(h)

	var hashIter C.GHashTableIter
	var key, value C.gpointer

	ret := make(map[string]string)
	C.g_hash_table_iter_init(&hashIter, h)
	for glib.GoBool(glib.GBoolean(C.g_hash_table_iter_next(&hashIter, &key, &value))) {
		ret[C.GoString((*C.char)(key))] = C.GoString((*C.char)(value))
	}
	return ret, nil
}

func (repo *OSTreeRepo) deleteBranch(branch string) error {
	var cerr *C.GError

	var remote *C.char
	var ref *C.char

	cBranch := C.CString(branch)
	defer C.free(unsafe.Pointer(cBranch))

	if !glib.GoBool(glib.GBoolean(C.ostree_parse_refspec(cBranch, &remote, &ref, &cerr))) {
		return glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	defer C.free(unsafe.Pointer(remote))
	defer C.free(unsafe.Pointer(ref))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_set_ref_immediate(repo.repo, remote, ref, nil, nil, &cerr))) {
		return glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return nil
}
