package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordLength(t *testing.T) {
	require.ErrorIs(t, CheckPasswordLength(strings.Repeat("a", PasswordMinLen-1)), ErrPasswordLength)
	require.NoError(t, CheckPasswordLength(strings.Repeat("a", PasswordMinLen)))
	require.NoError(t, CheckPasswordLength(strings.Repeat("a", PasswordMaxLen)))
	require.ErrorIs(t, CheckPasswordLength(strings.Repeat("a", PasswordMaxLen+1)), ErrPasswordLength)
	require.NotContains(t, ErrPasswordLength.Error(), strings.Repeat("a", PasswordMinLen-1))
}

func TestNormalizeUsername(t *testing.T) {
	username, err := NormalizeUsername("  reader  ")
	require.NoError(t, err)
	require.Equal(t, "reader", username)

	other, err := NormalizeUsername("Reader")
	require.NoError(t, err)
	require.NotEqual(t, username, other)

	cases := []string{"", " ", "has space", "bad/name", strings.Repeat("a", 65), "üser"}
	for _, raw := range cases {
		_, err := NormalizeUsername(raw)
		require.ErrorIs(t, err, ErrUsername)
	}
}

func TestHashPasswordUsesArgon2id(t *testing.T) {
	const password = "twelve-chars"
	const wrong = "wrong-password"

	first, err := HashPassword(password)
	require.NoError(t, err)
	second, err := HashPassword(password)
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=2$"))
	require.True(t, strings.HasPrefix(second, "$argon2id$v=19$m=65536,t=3,p=2$"))
	require.NotContains(t, first, password)
	require.NotContains(t, second, password)

	ok, err := VerifyPassword(first, password)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = VerifyPassword(first, wrong)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = VerifyPassword(second, wrong)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = VerifyPassword("not-a-hash", password)
	require.Error(t, err)
	require.False(t, ok)
	require.NotContains(t, err.Error(), password)

	_, err = HashPassword("short")
	require.ErrorIs(t, err, ErrPasswordLength)
}
