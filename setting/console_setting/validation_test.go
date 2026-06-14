package console_setting

import (
	"strings"
	"testing"
)

func TestValidateAnnouncementsCountsUnicodeCharacters(t *testing.T) {
	content := "<p>" + strings.Repeat("中", 490) + "</p>"
	announcements := `[{"content":"` + content + `","publishDate":"2026-06-14T00:00:00Z","type":"default"}]`

	if err := validateAnnouncements(announcements); err != nil {
		t.Fatalf("expected Chinese announcement under 500 characters to pass, got %v", err)
	}
}

func TestValidateAnnouncementsRejectsOverCharacterLimit(t *testing.T) {
	content := strings.Repeat("中", 501)
	announcements := `[{"content":"` + content + `","publishDate":"2026-06-14T00:00:00Z","type":"default"}]`

	if err := validateAnnouncements(announcements); err == nil {
		t.Fatal("expected announcement over 500 characters to fail")
	}
}

func TestValidateAnnouncementsExtraCountsUnicodeCharacters(t *testing.T) {
	extra := strings.Repeat("说", 200)
	announcements := `[{"content":"公告","publishDate":"2026-06-14T00:00:00Z","type":"default","extra":"` + extra + `"}]`

	if err := validateAnnouncements(announcements); err != nil {
		t.Fatalf("expected Chinese extra at 200 characters to pass, got %v", err)
	}
}
