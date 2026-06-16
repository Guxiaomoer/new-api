package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/community_sync_setting"
	"gorm.io/gorm"
)

const communitySyncPageLimit = 30

type CommunityMember struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	User   struct {
		Name     *string `json:"name"`
		Username string  `json:"username"`
	} `json:"user"`
}

type CommunitySyncResult struct {
	DryRun            bool     `json:"dry_run"`
	MemberCount       int      `json:"member_count"`
	LocalUserCount    int      `json:"local_user_count"`
	RestrictedCount   int      `json:"restricted_count"`
	UnrestrictedCount int      `json:"unrestricted_count"`
	ProtectedSkipped  int      `json:"protected_skipped"`
	RestrictUsers     []string `json:"restrict_users"`
	UnrestrictUsers   []string `json:"unrestrict_users"`
	Error             string   `json:"error,omitempty"`
}

type communityMembersRequest struct {
	RoomID  string `json:"roomId"`
	Limit   int    `json:"limit"`
	UntilID string `json:"untilId,omitempty"`
}

var communitySyncLoopOnce sync.Once

func FetchCommunityMembers(ctx context.Context, cfg community_sync_setting.CommunitySyncSetting) ([]CommunityMember, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("社区群成员接口地址不能为空")
	}
	if strings.TrimSpace(cfg.RoomID) == "" {
		return nil, errors.New("社区群 roomId 不能为空")
	}
	if strings.TrimSpace(cfg.Authorization) == "" {
		return nil, errors.New("社区群接口 Authorization token 不能为空")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	members := make([]CommunityMember, 0, communitySyncPageLimit)
	untilID := ""
	seenPages := 0

	for {
		seenPages++
		if seenPages > 200 {
			return nil, errors.New("社区群成员分页超过 200 页，已停止以避免死循环")
		}

		payload := communityMembersRequest{RoomID: cfg.RoomID, Limit: communitySyncPageLimit, UntilID: untilID}
		body, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.Authorization))
		if strings.TrimSpace(cfg.Fingerprint) != "" {
			req.Header.Set("x-client-fingerprint", strings.TrimSpace(cfg.Fingerprint))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("社区群成员接口返回 HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		var page []CommunityMember
		if err := common.Unmarshal(respBody, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		members = append(members, page...)
		if len(page) < communitySyncPageLimit {
			break
		}
		lastID := strings.TrimSpace(page[len(page)-1].ID)
		if lastID == "" || lastID == untilID {
			break
		}
		untilID = lastID
	}

	return members, nil
}

func PreviewCommunitySync(ctx context.Context) (*CommunitySyncResult, error) {
	return runCommunitySync(ctx, true)
}

func RunCommunitySync(ctx context.Context) (*CommunitySyncResult, error) {
	return runCommunitySync(ctx, false)
}

func runCommunitySync(ctx context.Context, dryRun bool) (*CommunitySyncResult, error) {
	cfg := community_sync_setting.Get()
	members, err := FetchCommunityMembers(ctx, cfg)
	if err != nil {
		return nil, err
	}

	memberSet := buildMemberIdentitySet(members)
	protectedSet := buildProtectedSet(cfg.ProtectedUsers)

	var users []model.User
	if err := model.DB.Where("deleted_at IS NULL").Find(&users).Error; err != nil {
		return nil, err
	}

	result := &CommunitySyncResult{DryRun: dryRun, MemberCount: len(memberSet), LocalUserCount: len(users)}
	type change struct {
		user       model.User
		restricted bool
	}
	changes := make([]change, 0)

	for _, user := range users {
		identities := localUserIdentities(user)
		if hasIntersection(identities, protectedSet) {
			result.ProtectedSkipped++
			continue
		}

		inCommunity := hasIntersection(identities, memberSet)
		setting := user.GetSetting()
		currentlyRestricted := setting.ApiRestricted

		if inCommunity && currentlyRestricted {
			result.UnrestrictedCount++
			result.UnrestrictUsers = append(result.UnrestrictUsers, user.Username)
			changes = append(changes, change{user: user, restricted: false})
		}
		if !inCommunity && !currentlyRestricted {
			result.RestrictedCount++
			result.RestrictUsers = append(result.RestrictUsers, user.Username)
			changes = append(changes, change{user: user, restricted: true})
		}
	}
	sort.Strings(result.RestrictUsers)
	sort.Strings(result.UnrestrictUsers)

	if dryRun || len(changes) == 0 {
		return result, nil
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range changes {
			setting := item.user.GetSetting()
			setting.ApiRestricted = item.restricted
			if !item.restricted {
				setting.ApiRestrictedMessage = ""
			}
			updatedSetting, err := marshalUserSetting(setting)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.User{}).Where("id = ?", item.user.Id).Update("setting", updatedSetting).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	for _, item := range changes {
		if err := model.InvalidateUserCache(item.user.Id); err != nil {
			common.SysLog(fmt.Sprintf("community sync: failed to invalidate user cache for %d: %s", item.user.Id, err.Error()))
		}
		if err := model.InvalidateUserTokensCache(item.user.Id); err != nil {
			common.SysLog(fmt.Sprintf("community sync: failed to invalidate tokens cache for %d: %s", item.user.Id, err.Error()))
		}
	}

	return result, nil
}

func StartCommunitySyncLoop() {
	if !common.IsMasterNode {
		return
	}
	communitySyncLoopOnce.Do(func() {
		go func() {
			ticker := time.NewTimer(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cfg := community_sync_setting.Get()
				if cfg.Enabled {
					result, err := RunCommunitySync(context.Background())
					if err != nil {
						common.SysLog("community sync failed: " + err.Error())
					} else {
						common.SysLog(fmt.Sprintf("community sync finished: members=%d local_users=%d restrict=%d unrestrict=%d protected=%d", result.MemberCount, result.LocalUserCount, result.RestrictedCount, result.UnrestrictedCount, result.ProtectedSkipped))
					}
				}

				interval := cfg.IntervalMinutes
				if interval < 1 {
					interval = community_sync_setting.DefaultIntervalMinute
				}
				ticker.Reset(time.Duration(interval) * time.Minute)
			}
		}()
	})
}

func buildMemberIdentitySet(members []CommunityMember) map[string]bool {
	set := map[string]bool{}
	for _, member := range members {
		addIdentity(set, member.User.Username)
		if member.User.Name != nil {
			addIdentity(set, *member.User.Name)
		}
	}
	return set
}

func buildProtectedSet(users []string) map[string]bool {
	set := map[string]bool{}
	for _, user := range users {
		addIdentity(set, user)
	}
	return set
}

func localUserIdentities(user model.User) map[string]bool {
	set := map[string]bool{}
	addIdentity(set, user.Username)
	addIdentity(set, user.DisplayName)
	addIdentity(set, user.Email)
	return set
}

func addIdentity(set map[string]bool, value string) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized != "" {
		set[normalized] = true
	}
}

func hasIntersection(left map[string]bool, right map[string]bool) bool {
	for key := range left {
		if right[key] {
			return true
		}
	}
	return false
}

func marshalUserSetting(setting dto.UserSetting) (string, error) {
	bytes, err := common.Marshal(setting)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
