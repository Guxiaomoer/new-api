package ratio_setting

import "testing"

func resetCompletionRatiosForTest(t *testing.T) {
	t.Helper()
	completionRatioMap.Clear()
	completionRatioMap.AddAll(defaultCompletionRatio)
}

func TestGetCompletionRatio_HardcodedModelWithoutOverride(t *testing.T) {
	resetCompletionRatiosForTest(t)

	if got := GetCompletionRatio("claude-opus-4-6"); got != 5 {
		t.Fatalf("GetCompletionRatio() = %v, want 5", got)
	}
}

func TestGetCompletionRatio_HardcodedModelWithOverride(t *testing.T) {
	resetCompletionRatiosForTest(t)
	completionRatioMap.Set("claude-opus-4-6", 1)

	if got := GetCompletionRatio("claude-opus-4-6"); got != 1 {
		t.Fatalf("GetCompletionRatio() = %v, want 1", got)
	}
}

func TestGetCompletionRatioInfo_OverrideUnlocksHardcodedModel(t *testing.T) {
	resetCompletionRatiosForTest(t)
	completionRatioMap.Set("gpt-5", 1)

	got := GetCompletionRatioInfo("gpt-5")
	if got.Ratio != 1 {
		t.Fatalf("GetCompletionRatioInfo().Ratio = %v, want 1", got.Ratio)
	}
	if got.Locked {
		t.Fatal("GetCompletionRatioInfo().Locked = true, want false for configured override")
	}
}

func TestGetCompletionRatioInfo_HardcodedModelWithoutOverrideStaysLocked(t *testing.T) {
	resetCompletionRatiosForTest(t)

	got := GetCompletionRatioInfo("gpt-5")
	if got.Ratio != 8 {
		t.Fatalf("GetCompletionRatioInfo().Ratio = %v, want 8", got.Ratio)
	}
	if !got.Locked {
		t.Fatal("GetCompletionRatioInfo().Locked = false, want true for hardcoded default")
	}
}

func TestGetCompletionRatio_NonClaudeOpenAIHardcodedModelIgnoresOverride(t *testing.T) {
	resetCompletionRatiosForTest(t)
	completionRatioMap.Set("mistral-large-latest", 1)

	if got := GetCompletionRatio("mistral-large-latest"); got != 3 {
		t.Fatalf("GetCompletionRatio() = %v, want 3", got)
	}
}

func TestGetCompletionRatioInfo_NonClaudeOpenAIHardcodedModelStaysLockedWithOverride(t *testing.T) {
	resetCompletionRatiosForTest(t)
	completionRatioMap.Set("mistral-large-latest", 1)

	got := GetCompletionRatioInfo("mistral-large-latest")
	if got.Ratio != 3 {
		t.Fatalf("GetCompletionRatioInfo().Ratio = %v, want 3", got.Ratio)
	}
	if !got.Locked {
		t.Fatal("GetCompletionRatioInfo().Locked = false, want true for non-Claude/OpenAI hardcoded model")
	}
}

func TestGetCompletionRatio_DefaultCompletionRatioDoesNotOverrideHardcoded(t *testing.T) {
	resetCompletionRatiosForTest(t)

	if got := GetCompletionRatio("gpt-image-1"); got != 2 {
		t.Fatalf("GetCompletionRatio() = %v, want hardcoded ratio 2", got)
	}
}
