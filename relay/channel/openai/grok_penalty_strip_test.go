package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func TestConvertOpenAIRequest_StripsPenaltiesForGrok(t *testing.T) {
	a := &Adaptor{}
	req := &dto.GeneralOpenAIRequest{
		Model:            "grok-4.5",
		FrequencyPenalty: float64Ptr(0),
		PresencePenalty:  float64Ptr(0.1),
		Temperature:      float64Ptr(0.7),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-4.5",
	}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:       1,
		UpstreamModelName: "grok-4.5",
	}

	out, err := a.ConvertOpenAIRequest(nil, info, req)
	require.NoError(t, err)
	got := out.(*dto.GeneralOpenAIRequest)
	require.Nil(t, got.FrequencyPenalty)
	require.Nil(t, got.PresencePenalty)
	require.NotNil(t, got.Temperature)
}

func TestConvertOpenAIRequest_KeepsPenaltiesForNonGrok(t *testing.T) {
	a := &Adaptor{}
	req := &dto.GeneralOpenAIRequest{
		Model:            "gpt-4o",
		FrequencyPenalty: float64Ptr(0),
		PresencePenalty:  float64Ptr(0.1),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
	}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:       1,
		UpstreamModelName: "gpt-4o",
	}

	out, err := a.ConvertOpenAIRequest(nil, info, req)
	require.NoError(t, err)
	got := out.(*dto.GeneralOpenAIRequest)
	require.NotNil(t, got.FrequencyPenalty)
	require.NotNil(t, got.PresencePenalty)
}
