package util

import (
	"strings"
	"testing"
)

func TestFormatDiff(t *testing.T) {
	input := "--- /tmp/old 2026-07-04\n+++ /tmp/new 2026-07-04\n@@ -1,3 +1,3 @@\n-old line\n+new line\n normal line"
	got := FormatDiff(input)

	if !strings.Contains(got, "[yellow]--- local/PKGBUILD[-]") {
		t.Errorf("expected local/PKGBUILD replacement, got: %s", got)
	}
	if !strings.Contains(got, "[yellow]+++ remote/PKGBUILD[-]") {
		t.Errorf("expected remote/PKGBUILD replacement, got: %s", got)
	}
	if !strings.Contains(got, "[teal]@@ -1,3 +1,3 @@[-]") {
		t.Errorf("expected cyan chunk header, got: %s", got)
	}
	if !strings.Contains(got, "[red]-old line[-]") {
		t.Errorf("expected red deletion, got: %s", got)
	}
	if !strings.Contains(got, "[green]+new line[-]") {
		t.Errorf("expected green addition, got: %s", got)
	}
	if !strings.Contains(got, "normal line") {
		t.Errorf("expected normal line preserved, got: %s", got)
	}
}

func TestFormatPKGBUILD(t *testing.T) {
	input := "# This is a comment\npkgname=test-pkg\npkgver=\"1.0.0\"\noptions=('!strip')"
	got := FormatPKGBUILD(input)

	if !strings.Contains(got, "[gray]# This is a comment[-]") {
		t.Errorf("expected gray comment, got: %s", got)
	}
	if !strings.Contains(got, "[teal]pkgname[-][fuchsia]=[-]test[fuchsia]-[-]pkg") {
		t.Errorf("expected cyan pkgname variable, got: %s", got)
	}
	if !strings.Contains(got, `[teal]pkgver[-][fuchsia]=[-][yellow]"[-][yellow]1.0.0[-][yellow]"[-]`) {
		t.Errorf("expected green string for pkgver value, got: %s", got)
	}
	if !strings.Contains(got, `[teal]options[-][fuchsia]=[-][fuchsia]([-][yellow]'!strip'[-][fuchsia])[-]`) {
		t.Errorf("expected green string for options single-quoted value, got: %s", got)
	}
}
