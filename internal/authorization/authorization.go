// Package authorization describes credential evidence and the CLI's permission to attempt mutations.
package authorization

import (
	"encoding/json"
	"slices"
	"strings"
)

type Metadata struct {
	Version         int      `json:"version"`
	Mode            string   `json:"mode"`
	Origin          string   `json:"origin"`
	RequestedScopes []string `json:"requested_scopes"`
	EffectiveScopes []string `json:"effective_scopes"`
	ScopeEvidence   string   `json:"scope_evidence"`
}

type Report struct {
	MetadataVersion       *int     `json:"metadata_version"`
	MetadataStatus        string   `json:"metadata_status"`
	Mode                  *string  `json:"mode"`
	Origin                string   `json:"origin"`
	RequestedScopes       []string `json:"requested_scopes"`
	EffectiveScopes       []string `json:"effective_scopes"`
	ScopeEvidence         string   `json:"scope_evidence"`
	WriteCapable          bool     `json:"write_capable"`
	WriteCapabilityReason string   `json:"write_capability_reason"`
}

func Resolve(raw json.RawMessage, source string, configured bool) Report {
	r := Report{MetadataStatus: "missing", Origin: "unknown", ScopeEvidence: "none", WriteCapabilityReason: "no-credential"}
	if !configured {
		return r
	}
	if source == "env" || len(raw) == 0 {
		mode := "unknown"
		r.Mode = &mode
		r.MetadataStatus = "legacy"
		if source == "env" {
			r.MetadataStatus = "external"
		}
		r.WriteCapable = true
		r.WriteCapabilityReason = "unknown-compatibility"
		return r
	}
	r.MetadataStatus = "invalid"
	r.WriteCapabilityReason = "invalid-metadata"
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return r
	}
	var version int
	if v, ok := fields["version"]; !ok || string(v) == "null" || json.Unmarshal(v, &version) != nil {
		return r
	}
	r.MetadataVersion = &version
	if version != 1 {
		r.MetadataStatus = "unsupported"
		r.WriteCapabilityReason = "unsupported-metadata"
		return r
	}
	for _, key := range []string{"mode", "origin", "requested_scopes", "effective_scopes", "scope_evidence"} {
		if _, ok := fields[key]; !ok {
			return r
		}
	}
	var m Metadata
	if json.Unmarshal(raw, &m) != nil || !m.valid() {
		return r
	}
	r.MetadataStatus = "valid"
	r.Mode = &m.Mode
	r.Origin = m.Origin
	r.RequestedScopes = m.RequestedScopes
	r.EffectiveScopes = m.EffectiveScopes
	r.ScopeEvidence = m.ScopeEvidence
	r.WriteCapable = m.Mode != "read-only"
	r.WriteCapabilityReason = m.Mode
	if m.Mode == "unknown" {
		r.WriteCapabilityReason = "unknown-compatibility"
	}
	return r
}

func (m Metadata) valid() bool {
	if m.Mode == "unknown" {
		return (m.Origin == "manual" || m.Origin == "unknown") && m.RequestedScopes == nil && m.EffectiveScopes == nil && m.ScopeEvidence == "none"
	}
	if m.Origin != "oauth-pkce" && m.Origin != "oauth-device" {
		return false
	}
	if m.ScopeEvidence != "token-response" && m.ScopeEvidence != "oauth-request" {
		return false
	}
	requestedMode := scopeMode(m.RequestedScopes)
	if requestedMode == "" || scopeMode(m.EffectiveScopes) != m.Mode || m.Mode == "" {
		return false
	}
	if requestedMode == "read-only" && m.Mode != "read-only" {
		return false
	}
	if requestedMode == "read-write" && !sameScopes(m.RequestedScopes, RequestedScopes(false)) {
		return false
	}
	return m.ScopeEvidence != "oauth-request" || sameScopes(m.RequestedScopes, m.EffectiveScopes)
}

func sameScopes(a, b []string) bool {
	a = slices.Clone(a)
	b = slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func scopeMode(scopes []string) string {
	if len(scopes) == 1 && scopes[0] == "data:read" {
		return "read-only"
	}
	seen := map[string]bool{}
	for _, s := range scopes {
		if seen[s] {
			return ""
		}
		seen[s] = true
		switch s {
		case "data:read", "data:read_write", "task:add", "data:delete", "project:delete":
		default:
			return ""
		}
	}
	if seen["data:read_write"] {
		return "read-write"
	}
	return ""
}

func RequestedScopes(readOnly bool) []string {
	if readOnly {
		return []string{"data:read"}
	}
	return []string{"data:read_write", "data:delete", "project:delete"}
}

func (r Report) Summary() string {
	if r.MetadataStatus == "invalid" || r.MetadataStatus == "unsupported" {
		return r.MetadataStatus + " authorization metadata; authenticated operations blocked"
	}
	if r.Mode == nil {
		return "no credential; writes blocked"
	}
	if *r.Mode == "unknown" {
		return "unknown; writes allowed for compatibility; scopes unknown; origin: " + r.Origin + "; evidence: none"
	}
	permission := "writes blocked"
	if r.WriteCapable {
		permission = "write attempts allowed"
	}
	return *r.Mode + "; " + permission + "; scopes: " + strings.Join(r.EffectiveScopes, ",") + "; origin: " + r.Origin + "; evidence: " + r.ScopeEvidence
}

// Error is a stable, secret-free authorization failure.
type Error struct {
	Code    string
	Message string
	Reason  string
}

func (e *Error) Error() string { return e.Message }

func (r Report) CheckCredential() error {
	switch r.MetadataStatus {
	case "invalid":
		return &Error{Code: "AUTH_METADATA_INVALID", Message: "Stored authorization metadata is invalid; log in again or remove the affected profile."}
	case "unsupported":
		return &Error{Code: "AUTH_METADATA_UNSUPPORTED", Message: "Stored authorization metadata uses an unsupported version; upgrade the CLI or replace the affected credential."}
	}
	return nil
}

func (r Report) CheckMutation() error {
	if err := r.CheckCredential(); err != nil {
		return err
	}
	if r.WriteCapable {
		return nil
	}
	return &Error{Code: "READ_ONLY", Message: "Todoist mutation blocked: the active credential is read-only."}
}

func OAuthMetadata(readOnly bool, origin string, returned json.RawMessage) (Metadata, error) {
	requested := RequestedScopes(readOnly)
	effective := slices.Clone(requested)
	evidence := "oauth-request"
	if len(returned) > 0 {
		var scope string
		if string(returned) == "null" || json.Unmarshal(returned, &scope) != nil {
			return Metadata{}, scopeError("unusable-grant")
		}
		effective = strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
		evidence = "token-response"
	}
	m := Metadata{Version: 1, Mode: scopeMode(effective), Origin: origin, RequestedScopes: requested, EffectiveScopes: effective, ScopeEvidence: evidence}
	if readOnly && m.Mode == "read-write" {
		return Metadata{}, scopeError("broader-than-requested")
	}
	if !m.valid() {
		return Metadata{}, scopeError("unsupported-grant")
	}
	return m, nil
}

func scopeError(reason string) error {
	return &Error{Code: "OAUTH_SCOPE_INVALID", Message: "OAuth returned an unacceptable scope grant; the stored credential was not changed.", Reason: reason}
}

func ManualMetadata() Metadata {
	return Metadata{Version: 1, Mode: "unknown", Origin: "manual", ScopeEvidence: "none"}
}
