package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// withPollutionKeywords 临时替换全局污染关键词配置，t.Cleanup 自动还原。
func withPollutionKeywords(t *testing.T, kw string) {
	t.Helper()
	general := operation_setting.GetGeneralSetting()
	old := general.UpstreamPollutionKeywords
	general.UpstreamPollutionKeywords = kw
	t.Cleanup(func() {
		general.UpstreamPollutionKeywords = old
	})
}

func TestDetectUpstreamPollutionHitsKeyword(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"resp_1","choices":[{"message":{"content":"通▸知◁群 175877552 加群送福利"}}]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "通▸知◁群", hit)
}

func TestDetectUpstreamPollutionHitsFakeId(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"chatcmpl_local_abcdef","choices":[]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "chatcmpl_local_", hit)
}

func TestDetectUpstreamPollutionMissesCleanResponse(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"chatcmpl-abc","choices":[{"message":{"content":"Hello, how can I help you?"}}]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "", hit)
}

func TestDetectUpstreamPollutionEmptyConfig(t *testing.T) {
	withPollutionKeywords(t, "")

	body := []byte(`{"content":"通▸知◁群 加群送福利"}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "", hit, "空配置应跳过检测,即使响应体含敏感词")
}

func TestDetectUpstreamPollutionEmptyBody(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群")

	hit := DetectUpstreamPollution(nil)
	require.Equal(t, "", hit)
	hit = DetectUpstreamPollution([]byte{})
	require.Equal(t, "", hit)
}

func TestDetectUpstreamPollutionStringVariant(t *testing.T) {
	withPollutionKeywords(t, "公益 token")

	hit := DetectUpstreamPollutionString("data: {\"content\":\"公益 token 先休息10分钟\"}\n")
	require.Equal(t, "公益 token", hit)
}

func TestGetUpstreamPollutionKeywordsTrimsAndDedupes(t *testing.T) {
	withPollutionKeywords(t, "  通▸知◁群  \n\n公益 token\n通▸知◁群\n  \n")

	keywords := operation_setting.GetUpstreamPollutionKeywords()
	require.Equal(t, []string{"通▸知◁群", "公益 token"}, keywords,
		"应当去除空白行、首尾空格、重复条目")
}

func TestDetectUpstreamPollutionHandlesLargeBody(t *testing.T) {
	withPollutionKeywords(t, "公益 token")

	// 模拟前面有大段正常内容，末尾才出现污染关键词
	body := strings.Repeat("a", 8*1024) + "公益 token here"
	hit := DetectUpstreamPollution([]byte(body))
	require.Equal(t, "公益 token", hit, "应当扫描完整 body,不限制开头位置")
}
