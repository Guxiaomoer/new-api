package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/community_sync_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFetchCommunityMembersUsesUntilIDPagination(t *testing.T) {
	requests := make([]communityMembersRequest, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "fingerprint-1", r.Header.Get("x-client-fingerprint"))

		var req communityMembersRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		requests = append(requests, req)

		if req.UntilID == "" {
			members := make([]CommunityMember, communitySyncPageLimit)
			for i := range members {
				members[i].ID = "member-page-1"
				members[i].User.Username = "page1-user"
			}
			members[len(members)-1].ID = "cursor-1"
			body, err := common.Marshal(members)
			require.NoError(t, err)
			_, _ = w.Write(body)
			return
		}

		require.Equal(t, "cursor-1", req.UntilID)
		members := []CommunityMember{{ID: "member-page-2"}}
		members[0].User.Username = "page2-user"
		body, err := common.Marshal(members)
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	members, err := FetchCommunityMembers(context.Background(), community_sync_setting.CommunitySyncSetting{
		Endpoint:      server.URL,
		RoomID:        "room-1",
		Authorization: "test-token",
		Fingerprint:   "fingerprint-1",
	})

	require.NoError(t, err)
	require.Len(t, members, communitySyncPageLimit+1)
	require.Len(t, requests, 2)
	require.Equal(t, "room-1", requests[0].RoomID)
	require.Equal(t, communitySyncPageLimit, requests[0].Limit)
	require.Empty(t, requests[0].UntilID)
	require.Equal(t, "cursor-1", requests[1].UntilID)
}

func TestRunCommunitySyncRestrictsOnlyUsersOutsideCommunityAndSkipsProtected(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	require.NoError(t, db.AutoMigrate(&model.User{}))

	restricted := dto.UserSetting{ApiRestricted: true}
	restrictedBytes, err := common.Marshal(restricted)
	require.NoError(t, err)

	require.NoError(t, db.Create(&model.User{Username: "member", DisplayName: "Community User", Email: "member@example.com", Setting: string(restrictedBytes)}).Error)
	require.NoError(t, db.Create(&model.User{Username: "outsider", DisplayName: "Outsider", Email: "outsider@example.com"}).Error)
	require.NoError(t, db.Create(&model.User{Username: "1456671048@qq.com", DisplayName: "Root", Email: "1456671048@qq.com"}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		members := []CommunityMember{{ID: "m1"}}
		members[0].User.Username = "member"
		body, err := common.Marshal(members)
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	community_sync_setting.Update("community_sync.endpoint", server.URL)
	community_sync_setting.Update("community_sync.room_id", "room-1")
	community_sync_setting.Update("community_sync.authorization", "test-token")
	community_sync_setting.Update("community_sync.protected_users", "1456671048@qq.com\nlufeng2820@163.com")

	result, err := RunCommunitySync(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.MemberCount)
	require.Equal(t, 3, result.LocalUserCount)
	require.Equal(t, 1, result.RestrictedCount)
	require.Equal(t, 1, result.UnrestrictedCount)
	require.Equal(t, 1, result.ProtectedSkipped)

	var member model.User
	require.NoError(t, db.Where("username = ?", "member").First(&member).Error)
	require.False(t, member.GetSetting().ApiRestricted)

	var outsider model.User
	require.NoError(t, db.Where("username = ?", "outsider").First(&outsider).Error)
	require.True(t, outsider.GetSetting().ApiRestricted)

	var root model.User
	require.NoError(t, db.Where("username = ?", "1456671048@qq.com").First(&root).Error)
	require.False(t, root.GetSetting().ApiRestricted)
}
