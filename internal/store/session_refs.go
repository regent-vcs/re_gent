package store

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

const encodedSessionPrefix = "~b64~"

// SessionRefPath returns the ref path used for a logical session ID.
func SessionRefPath(sessionID string) string {
	return filepath.ToSlash(filepath.Join("sessions", encodeSessionRefName(sessionID)))
}

// ReadSessionRef reads the session head ref for a logical session ID.
func (s *Store) ReadSessionRef(sessionID string) (Hash, error) {
	return s.ReadRef(SessionRefPath(sessionID))
}

// UpdateSessionRef updates the session head ref for a logical session ID.
func (s *Store) UpdateSessionRef(sessionID string, expectedOld, newHash Hash) error {
	return s.UpdateRef(SessionRefPath(sessionID), expectedOld, newHash)
}

// UpdateSessionRefWithRetry updates the session head ref with retry semantics.
func (s *Store) UpdateSessionRefWithRetry(sessionID string, expectedOld, newHash Hash, maxAttempts int) error {
	return s.UpdateRefWithRetry(SessionRefPath(sessionID), expectedOld, newHash, maxAttempts)
}

// DeleteSessionRef deletes the session head ref for a logical session ID.
func (s *Store) DeleteSessionRef(sessionID string, expectedOld Hash) error {
	return s.DeleteRef(SessionRefPath(sessionID), expectedOld)
}

// ListSessionRefs returns all session refs keyed by logical session ID.
func (s *Store) ListSessionRefs() (map[string]Hash, error) {
	refs, err := s.ListRefs("sessions")
	if err != nil {
		return nil, err
	}

	decoded := make(map[string]Hash, len(refs))
	for name, hash := range refs {
		decoded[decodeSessionRefName(name)] = hash
	}
	return decoded, nil
}

func encodeSessionRefName(sessionID string) string {
	if isSafeSessionRefName(sessionID) {
		return sessionID
	}
	return encodedSessionPrefix + base64.RawURLEncoding.EncodeToString([]byte(sessionID))
}

func decodeSessionRefName(name string) string {
	if !strings.HasPrefix(name, encodedSessionPrefix) {
		return name
	}

	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(name, encodedSessionPrefix))
	if err != nil {
		return name
	}
	return string(decoded)
}

func isSafeSessionRefName(sessionID string) bool {
	if sessionID == "" {
		return false
	}

	for _, r := range sessionID {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}

	return true
}
