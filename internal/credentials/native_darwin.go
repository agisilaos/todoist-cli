//go:build darwin && cgo

package credentials

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#cgo CFLAGS: -Wno-deprecated-declarations
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

enum CredentialOperation { CredentialProbe, CredentialWrite, CredentialRead, CredentialDelete };

// Every operation uses an explicit keychain reference and restores the process
// interaction setting. Go serializes this scope because the setting is global.
static OSStatus credential_operation(const char *path, const char *service,
 const char *account, const void *secret, UInt32 secretLen, int operation,
 void **result, UInt32 *resultLen) {
 Boolean previous;
 OSStatus status = SecKeychainGetUserInteractionAllowed(&previous);
 if (status != errSecSuccess) return status;
 status = SecKeychainSetUserInteractionAllowed(false);
 if (status != errSecSuccess) return status;
 SecKeychainRef keychain = NULL;
 SecKeychainItemRef item = NULL;
 void *content = NULL;
 UInt32 length = 0;
 if (path[0]) status = SecKeychainOpen(path, &keychain);
 else status = SecKeychainCopyDefault(&keychain);
 if (status == errSecSuccess && keychain == NULL) status = errSecNoSuchKeychain;
 if (status == errSecSuccess) {
  SecKeychainStatus flags;
  status = SecKeychainGetStatus(keychain, &flags);
  if (status == errSecSuccess && !(flags & kSecUnlockStateStatus)) status = -100001;
 }
 if (status == errSecSuccess && operation == CredentialWrite) {
  status = SecKeychainAddGenericPassword(keychain, (UInt32)strlen(service), service,
   (UInt32)strlen(account), account, secretLen, secret, NULL);
 } else if (status == errSecSuccess && (operation == CredentialRead || operation == CredentialDelete)) {
  status = SecKeychainFindGenericPassword(keychain, (UInt32)strlen(service), service,
   (UInt32)strlen(account), account, operation == CredentialRead ? &length : NULL,
   operation == CredentialRead ? &content : NULL, &item);
  if (status == errSecSuccess && operation == CredentialRead) {
   *result = malloc(length ? length : 1);
   if (*result == NULL) status = errSecAllocate;
   else { memcpy(*result, content, length); *resultLen = length; }
  }
  if (status == errSecSuccess && operation == CredentialDelete) status = SecKeychainItemDelete(item);
 }
 if (content) SecKeychainItemFreeContent(NULL, content);
 if (item) CFRelease(item);
 if (keychain) CFRelease(keychain);
 OSStatus restored = SecKeychainSetUserInteractionAllowed(previous);
 if (status == errSecSuccess) status = restored;
 return status;
}
*/
import "C"

import (
	"context"
	"sync"
	"unsafe"
)

var nativeMu sync.Mutex

type keychain struct{ path string }

// NewNative creates an adapter without accessing the user's Keychain.
func NewNative() Secrets { return &keychain{} }

func (k *keychain) operation(ctx context.Context, op int, account, token string) (string, error) {
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", failure(IO)
	}
	path := C.CString(k.path)
	defer C.free(unsafe.Pointer(path))
	service := C.CString(Service)
	defer C.free(unsafe.Pointer(service))
	entry := C.CString(account)
	defer C.free(unsafe.Pointer(entry))
	var data unsafe.Pointer
	if len(token) > 0 {
		data = C.CBytes([]byte(token))
		defer C.free(data)
	}
	var result unsafe.Pointer
	var size C.UInt32
	status := C.credential_operation(path, service, entry, data, C.UInt32(len(token)), C.int(op), &result, &size)
	if result != nil {
		defer C.free(result)
	}
	if err := nativeError(int(status)); err != nil {
		return "", err
	}
	if result != nil {
		return string(C.GoBytes(result, C.int(size))), nil
	}
	return "", nil
}
func (k *keychain) Read(ctx context.Context, id string) (string, error) {
	return k.operation(ctx, int(C.CredentialRead), id, "")
}
func (k *keychain) Write(ctx context.Context, id, token string) error {
	_, err := k.operation(ctx, int(C.CredentialWrite), id, token)
	return err
}
func (k *keychain) Delete(ctx context.Context, id string) error {
	_, err := k.operation(ctx, int(C.CredentialDelete), id, "")
	return err
}
func (k *keychain) Probe(ctx context.Context) error {
	_, err := k.operation(ctx, int(C.CredentialProbe), "", "")
	return err
}
func nativeError(status int) error {
	switch status {
	case 0:
		return nil
	case -25300:
		return failure(Missing)
	case -25308, -25315:
		return failure(Interaction)
	case -100001:
		return failure(Locked)
	case -25293, -25292, -128:
		return failure(Denied)
	case -25291, -25294, -25295, -25307:
		return failure(Unavailable)
	default:
		return failure(IO)
	}
}
