package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeLikePattern_AllowsWildcard(t *testing.T) {
	result, err := sanitizeLikePattern("test%value")
	require.NoError(t, err)
	require.Equal(t, "test%value", result)
}

func TestSanitizeLikePattern_EscapesUnderscore(t *testing.T) {
	result, err := sanitizeLikePattern("test_value")
	require.NoError(t, err)
	require.Equal(t, "test!_value", result)
}

func TestSanitizeLikePattern_EscapesBang(t *testing.T) {
	result, err := sanitizeLikePattern("test!value")
	require.NoError(t, err)
	require.Equal(t, "test!!value", result)
}

func TestSanitizeLikePattern_EscapesMultiple(t *testing.T) {
	result, err := sanitizeLikePattern("%test_!val%")
	require.NoError(t, err)
	require.Equal(t, "%test!_!!val%", result)
}

func TestSanitizeLikePattern_PlainString(t *testing.T) {
	result, err := sanitizeLikePattern("plain-value")
	require.NoError(t, err)
	require.Equal(t, "plain-value", result)
}

func TestSanitizeLikePattern_EmptyString(t *testing.T) {
	result, err := sanitizeLikePattern("")
	require.NoError(t, err)
	require.Equal(t, "", result)
}

func TestSanitizeLikePattern_TooManyWildcards(t *testing.T) {
	_, err := sanitizeLikePattern("%too%many%")
	require.Error(t, err)
}

func TestInterceptLogTableName(t *testing.T) {
	log := InterceptLog{}
	require.Equal(t, "intercept_logs", log.TableName())
}
