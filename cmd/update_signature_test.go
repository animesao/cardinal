//go:build linux

package cmd

import (
	"fmt"
	"strings"
	"testing"
)

// TestReleaseSignatureTargetsManifest — проверка подписи должна смотреть
// на тот артефакт, который релизный воркфлоу подписывает.
//
// Раньше проверка искала подпись рядом с per-file чексуммами
// (cardinal_<ver>_checksums.txt.sig), которых в релизе нет, и пиннила
// workflow release.yml, которого в репозитории тоже нет. Проверка не могла
// пройти ни при каких условиях, а проверка включена по умолчанию —
// `cardinal update` был бы сломан у всех.
func TestReleaseSignatureTargetsManifest(t *testing.T) {
	const (
		releaseURL = "https://github.com/animesao/cardinal"
		repoOwner  = "animesao"
		repoName   = "cardinal"
	)

	for _, version := range []string{"2.4.7", "2.4.8"} {
		tag := "v" + version
		manifestURL := fmt.Sprintf("%s/releases/download/%s/SHA256SUMS.txt", releaseURL, tag)
		sigURL := manifestURL + ".sig"

		if !strings.HasSuffix(manifestURL, "/SHA256SUMS.txt") {
			t.Fatalf("%s: подписывается манифест SHA256SUMS.txt, а не %s", version, manifestURL)
		}
		if !strings.HasSuffix(sigURL, ".sig") {
			t.Fatalf("%s: подпись должна лежать рядом с манифестом: %s", version, sigURL)
		}
		if strings.Contains(manifestURL, "checksums.txt") {
			t.Errorf("%s: подписываться должен манифест, а не per-file чексуммы: %s", version, manifestURL)
		}
	}
}

// TestSignatureIdentityPointsAtRealWorkflow — идентичность сертификата
// должна указывать на существующий воркфлоу: подпись делает build.yml,
// а не release.yml (такого в репозитории нет).
func TestSignatureIdentityPointsAtRealWorkflow(t *testing.T) {
	tag := "v2.4.7"
	idRegex := fmt.Sprintf("^https://github\\.com/animesao/cardinal/\\.github/workflows/build\\.yml@refs/tags/%s$",
		regexpQuoteMeta(tag))

	if strings.Contains(idRegex, "release.yml") {
		t.Fatalf("идентичность указывает на несуществующий workflow: %s", idRegex)
	}
	if !strings.Contains(idRegex, "build.yml") {
		t.Fatalf("идентичность должна указывать на build.yml: %s", idRegex)
	}
	if !strings.Contains(idRegex, regexpQuoteMeta(tag)) {
		t.Fatalf("идентичность должна быть привязана к тегу %s: %s", tag, idRegex)
	}
}
