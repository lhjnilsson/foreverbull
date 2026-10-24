package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	// PasswordMinLen and PasswordMaxLen are the login and account-command bounds.
	PasswordMinLen = 12
	PasswordMaxLen = 128

	// argon2id PHC parameters. m is KiB, so 65536 is 64 MiB.
	argonMemoryKiB   = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLen     = 16
	argonKeyLen      = 32

	// A stored hash selects the cost. These caps stop one row from
	// asking the process for unbounded memory or CPU.
	maxArgonMemoryKiB   = 256 * 1024
	maxArgonIterations  = 10
	maxArgonParallelism = 8
)

const dummyPassword = "foreverbull-dummy-password-not-used-as-a-login"

var (
	ErrPasswordLength  = errors.New("password length must be 12 to 128")
	ErrUsername        = errors.New("username must match ^[A-Za-z0-9._-]{1,64}$")
	ErrUsernameExists  = errors.New("username already exists")
	ErrAccountNotFound = errors.New("account not found")
	ErrUnauthorized    = errors.New("unauthorized")

	errInvalidHash = errors.New("password hash: invalid format")

	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

	dummyOnce    sync.Once
	dummyEncoded string
	dummyErr     error
)

// CheckPasswordLength accepts a password login will accept.
func CheckPasswordLength(password string) error {
	n := len(password)
	if n < PasswordMinLen || n > PasswordMaxLen {
		return ErrPasswordLength
	}
	return nil
}

// NormalizeUsername trims and checks the login name. Match is case-sensitive.
func NormalizeUsername(raw string) (string, error) {
	username := strings.TrimSpace(raw)
	if !usernamePattern.MatchString(username) {
		return "", ErrUsername
	}
	return username, nil
}

// HashPassword returns an argon2id PHC string. The password is not logged.
func HashPassword(password string) (string, error) {
	if err := CheckPasswordLength(password); err != nil {
		return "", err
	}
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemoryKiB,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	for i := range key {
		key[i] = 0
	}
	return encoded, nil
}

// VerifyPassword reports whether password matches an argon2id PHC string.
// A wrong password is false and a nil error. The password is not part of the error.
func VerifyPassword(encoded, password string) (bool, error) {
	salt, want, params, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want)))
	match := subtle.ConstantTimeCompare(got, want) == 1
	for i := range got {
		got[i] = 0
	}
	return match, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parsePHC(encoded string) ([]byte, []byte, argonParams, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, argonParams{}, errInvalidHash
	}
	if err := parseVersion(parts[2]); err != nil {
		return nil, nil, argonParams{}, err
	}

	params, err := parseArgonParams(parts[3])
	if err != nil {
		return nil, nil, argonParams{}, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return nil, nil, argonParams{}, errInvalidHash
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return nil, nil, argonParams{}, errInvalidHash
	}
	return salt, key, params, nil
}

func parseVersion(field string) error {
	if !strings.HasPrefix(field, "v=") {
		return errInvalidHash
	}
	version, err := strconv.Atoi(field[len("v="):])
	if err != nil || version != argon2.Version {
		return errInvalidHash
	}
	return nil
}

func parseArgonParams(field string) (argonParams, error) {
	var params argonParams
	var sawMemory, sawTime, sawThreads bool

	for _, item := range strings.Split(field, ",") {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			return argonParams{}, errInvalidHash
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return argonParams{}, errInvalidHash
		}

		switch name {
		case "m":
			if sawMemory || parsed == 0 || parsed > maxArgonMemoryKiB {
				return argonParams{}, errInvalidHash
			}
			params.memory = uint32(parsed)
			sawMemory = true
		case "t":
			if sawTime || parsed == 0 || parsed > maxArgonIterations {
				return argonParams{}, errInvalidHash
			}
			params.time = uint32(parsed)
			sawTime = true
		case "p":
			if sawThreads || parsed == 0 || parsed > maxArgonParallelism {
				return argonParams{}, errInvalidHash
			}
			params.threads = uint8(parsed)
			sawThreads = true
		default:
			return argonParams{}, errInvalidHash
		}
	}

	if !sawMemory || !sawTime || !sawThreads {
		return argonParams{}, errInvalidHash
	}
	return params, nil
}

// dummyHash is built once per process so a missing username pays the same
// argon2 cost as a stored hash. It is not a login.
func dummyHash() (string, error) {
	dummyOnce.Do(func() {
		dummyEncoded, dummyErr = hashPassword(dummyPassword)
	})
	return dummyEncoded, dummyErr
}
