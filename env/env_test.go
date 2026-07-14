package env_test

import (
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/env"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestGetDefault(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("set and non-empty returns the value", func(t *testing.T) {
		t.Setenv("EGL_STR", "hello")
		require.Equal(t, "hello", env.GetDefault("EGL_STR", "fb"))
	})
	t.Run("unset returns the fallback", func(t *testing.T) {
		require.Equal(t, "fb", env.GetDefault("EGL_STR_UNSET", "fb"))
	})
	t.Run("empty returns the fallback", func(t *testing.T) {
		t.Setenv("EGL_STR_EMPTY", "")
		require.Equal(t, "fb", env.GetDefault("EGL_STR_EMPTY", "fb"))
	})
}

func TestGetInt(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("valid", func(t *testing.T) {
		t.Setenv("EGL_INT", "42")
		require.Equal(t, 42, env.GetInt("EGL_INT", 7))
	})
	t.Run("negative", func(t *testing.T) {
		t.Setenv("EGL_INT", "-3")
		require.Equal(t, -3, env.GetInt("EGL_INT", 7))
	})
	t.Run("unset falls back", func(t *testing.T) {
		require.Equal(t, 7, env.GetInt("EGL_INT_UNSET", 7))
	})
	t.Run("malformed falls back", func(t *testing.T) {
		t.Setenv("EGL_INT", "notanint")
		require.Equal(t, 7, env.GetInt("EGL_INT", 7))
	})
}

func TestGetBool(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("true forms", func(t *testing.T) {
		for _, v := range []string{"1", "t", "TRUE", "true"} {
			t.Setenv("EGL_BOOL", v)
			require.True(t, env.GetBool("EGL_BOOL", false), v)
		}
	})
	t.Run("false form", func(t *testing.T) {
		t.Setenv("EGL_BOOL", "0")
		require.False(t, env.GetBool("EGL_BOOL", true))
	})
	t.Run("unset falls back", func(t *testing.T) {
		require.True(t, env.GetBool("EGL_BOOL_UNSET", true))
	})
	t.Run("malformed falls back", func(t *testing.T) {
		t.Setenv("EGL_BOOL", "maybe")
		require.True(t, env.GetBool("EGL_BOOL", true))
	})
}

func TestGetDuration(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("valid", func(t *testing.T) {
		t.Setenv("EGL_DUR", "1.5h")
		require.Equal(t, 90*time.Minute, env.GetDuration("EGL_DUR", time.Second))
	})
	t.Run("unset falls back", func(t *testing.T) {
		require.Equal(t, time.Second, env.GetDuration("EGL_DUR_UNSET", time.Second))
	})
	t.Run("malformed falls back", func(t *testing.T) {
		t.Setenv("EGL_DUR", "soon")
		require.Equal(t, time.Second, env.GetDuration("EGL_DUR", time.Second))
	})
}
