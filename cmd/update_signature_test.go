package cmd

import (
	"regexp"
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
	for _, version := range []string{"2.4.7", "2.4.8", "2.5.0"} {
		tag := "v" + version
		manifestURL := releaseManifestURL(tag)
		sigURL := manifestURL + ".sig"

		if !strings.HasSuffix(manifestURL, "/SHA256SUMS.txt") {
			t.Errorf("%s: подписывается манифест SHA256SUMS.txt, а не %s", version, manifestURL)
		}
		if !strings.HasSuffix(sigURL, ".sig") {
			t.Errorf("%s: подпись должна лежать рядом с манифестом: %s", version, sigURL)
		}
		if strings.Contains(manifestURL, "checksums.txt") {
			t.Errorf("%s: подписываться должен манифест, а не per-file чексуммы: %s", version, manifestURL)
		}
		if !strings.Contains(manifestURL, "/releases/download/"+tag+"/") {
			t.Errorf("%s: манифест должен качаться из релиза с тегом %s: %s", version, tag, manifestURL)
		}
	}
}

// TestSignatureIdentityPointsAtRealWorkflow — идентичность сертификата
// должна указывать на существующий воркфлоу: подпись делает build.yml,
// а не release.yml (такого в репозитории нет).
func TestSignatureIdentityPointsAtRealWorkflow(t *testing.T) {
	const tag = "v2.4.8"

	idRegex := cosignIdentityRegexp(tag)
	re, err := regexp.Compile(idRegex)
	if err != nil {
		t.Fatalf("идентичность собралась в невалидный regexp %q: %v", idRegex, err)
	}

	good := "https://github.com/animesao/cardinal/.github/workflows/build.yml@refs/tags/v2.4.8"
	if !re.MatchString(good) {
		t.Errorf("идентичность %q должна принимать %q", idRegex, good)
	}

	bad := []string{
		// несуществующий воркфлоу
		"https://github.com/animesao/cardinal/.github/workflows/release.yml@refs/tags/v2.4.8",
		// другой тег
		"https://github.com/animesao/cardinal/.github/workflows/build.yml@refs/tags/v2.4.7",
		// другой репозиторий
		"https://github.com/animesao/evil/.github/workflows/build.yml@refs/tags/v2.4.8",
		// ветка вместо тега
		"https://github.com/animesao/cardinal/.github/workflows/build.yml@refs/heads/main",
	}
	for _, id := range bad {
		if re.MatchString(id) {
			t.Errorf("идентичность %q не должна принимать %q", idRegex, id)
		}
	}
}
