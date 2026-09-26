//go:build darwin && cgo && credentialintegration

package credentials

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#cgo CFLAGS: -Wno-deprecated-declarations
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

// Private keychains never use a login.keychain path. No search-list/default
// setter is called. Snapshot invariants are checked around their lifecycle.
static OSStatus test_keychain(const char *path, const char *password, int operation) {
 Boolean previous;
 OSStatus status = SecKeychainGetUserInteractionAllowed(&previous);
 if (status) return status;
 status = SecKeychainSetUserInteractionAllowed(false);
 if (status) return status;
 SecKeychainRef keychain = NULL;
 CFArrayRef before = NULL, after = NULL;
 SecKeychainRef defaultBefore = NULL, defaultAfter = NULL;
 OSStatus beforeListStatus = SecKeychainCopySearchList(&before);
 OSStatus beforeDefaultStatus = SecKeychainCopyDefault(&defaultBefore);
 // A missing default is a known empty state; every other snapshot failure
 // prevents us from establishing isolation and must abort before mutation.
 int validBefore = beforeListStatus == 0 && before != NULL &&
  ((beforeDefaultStatus == 0 && defaultBefore != NULL) ||
   (beforeDefaultStatus == errSecNoDefaultKeychain && defaultBefore == NULL));
 if (!validBefore) status = -100002;
 else if (operation == 0) status = SecKeychainCreate(path, (UInt32)strlen(password), password, false, NULL, &keychain);
 else status = SecKeychainOpen(path, &keychain);
 if (status == 0 && operation == 1) status = SecKeychainLock(keychain);
 if (status == 0 && operation == 2) status = SecKeychainUnlock(keychain, (UInt32)strlen(password), password, true);
 if (status == 0 && operation == 3) status = SecKeychainDelete(keychain);
 OSStatus afterListStatus = SecKeychainCopySearchList(&after);
 OSStatus afterDefaultStatus = SecKeychainCopyDefault(&defaultAfter);
 int validAfter = afterListStatus == 0 && after != NULL &&
  ((afterDefaultStatus == 0 && defaultAfter != NULL) ||
   (afterDefaultStatus == errSecNoDefaultKeychain && defaultAfter == NULL));
 if (!validBefore || !validAfter || beforeListStatus != afterListStatus || beforeDefaultStatus != afterDefaultStatus ||
  (before && after && !CFEqual(before,after)) ||
  (defaultBefore && defaultAfter && !CFEqual(defaultBefore,defaultAfter))) status = -100002;
 if (before) CFRelease(before); if (after) CFRelease(after);
 if (defaultBefore) CFRelease(defaultBefore); if (defaultAfter) CFRelease(defaultAfter);
 if (keychain) CFRelease(keychain);
 OSStatus restored = SecKeychainSetUserInteractionAllowed(previous);
 if (status == 0) status = restored;
 return status;
}
*/
import "C"
import (
	"errors"
	"unsafe"
)

func isolatedKeychain(path, password string, operation int) error {
	nativeMu.Lock()
	defer nativeMu.Unlock()
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	secret := C.CString(password)
	defer C.free(unsafe.Pointer(secret))
	status := int(C.test_keychain(p, secret, C.int(operation)))
	if status == -100002 {
		return errors.New("isolated Keychain snapshots could not be established or changed unexpectedly")
	}
	return nativeError(status)
}
