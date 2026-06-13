package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserInsertStoresInviterId(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username: "invitee-normal",
		Password: "password123",
	}

	require.NoError(t, user.Insert(123))

	var created User
	require.NoError(t, DB.First(&created, "username = ?", user.Username).Error)
	require.Equal(t, 123, created.InviterId)
}

func TestUserInsertLeavesInviterIdEmptyWithoutInviter(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username: "invitee-none",
		Password: "password123",
	}

	require.NoError(t, user.Insert(0))

	var created User
	require.NoError(t, DB.First(&created, "username = ?", user.Username).Error)
	require.Zero(t, created.InviterId)
}

func TestUserInsertWithTxStoresInviterId(t *testing.T) {
	truncateTables(t)

	tx := DB.Begin()
	require.NoError(t, tx.Error)

	user := &User{
		Username: "invitee-oauth",
		Password: "",
	}

	require.NoError(t, user.InsertWithTx(tx, 456))
	require.NoError(t, tx.Commit().Error)

	var created User
	require.NoError(t, DB.First(&created, "username = ?", user.Username).Error)
	require.Equal(t, 456, created.InviterId)
}
