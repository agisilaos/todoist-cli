package credentials

import "errors"

type Kind string

const (
	Missing     Kind = "CREDENTIAL_MISSING"
	Unavailable Kind = "CREDENTIAL_STORE_UNAVAILABLE"
	Denied      Kind = "CREDENTIAL_STORE_DENIED"
	Interaction Kind = "CREDENTIAL_STORE_INTERACTION_REQUIRED"
	Locked      Kind = "CREDENTIAL_STORE_LOCKED"
	Corrupt     Kind = "CREDENTIAL_STORE_CORRUPT"
	Unsupported Kind = "CREDENTIAL_STORE_UNSUPPORTED"
	Namespace   Kind = "CREDENTIAL_STORE_NAMESPACE_MISMATCH"
	Busy        Kind = "CREDENTIAL_STORE_BUSY"
	IO          Kind = "CREDENTIAL_STORE_IO"
	Cleanup     Kind = "CREDENTIAL_CLEANUP_PENDING"
	Recovery    Kind = "CREDENTIAL_RECOVERY_REQUIRED"
	Selection   Kind = "CREDENTIAL_STORE_SELECTION_CONFLICT"
)

type Error struct {
	Kind      Kind
	Committed *bool
}

func (e *Error) Error() string {
	switch e.Kind {
	case Missing:
		return "The selected profile's native credential is missing; log in again."
	case Unavailable:
		return "Native credential storage is unavailable; explicitly select file storage or use a supported build."
	case Denied:
		return "Credential store access was denied; adjust access outside the CLI and retry."
	case Interaction:
		return "Credential store interaction is required; unlock or adjust Keychain outside the CLI and retry."
	case Locked:
		return "The credential store is locked; unlock it outside the CLI and retry."
	case Corrupt:
		return "Credential storage is corrupt; preserve the file and recover it explicitly."
	case Unsupported:
		return "Credential storage uses an unsupported format; upgrade the CLI."
	case Namespace:
		return "The profile belongs to another configuration directory; log in again here."
	case Busy:
		return "Credential storage is busy; retry after the other writer finishes."
	case Cleanup:
		return "The credential change completed, but native cleanup is pending; run 'todoist auth repair'."
	case Recovery:
		return "Credential recovery is required; run 'todoist auth repair' before changing this profile."
	case Selection:
		return "The selected profile uses a different backend; run 'todoist auth migrate' first."
	default:
		return "Credential storage could not complete an I/O operation."
	}
}
func failure(k Kind) *Error { return &Error{Kind: k} }

// Normalize discards untrusted adapter messages, including errors from fakes.
func Normalize(err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return &Error{Kind: e.Kind, Committed: e.Committed}
	}
	return failure(IO)
}
