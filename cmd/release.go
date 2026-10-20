// Package-level release endpoints and the pure helpers that describe how a
// release is signed.
//
// Этот файл намеренно без build-тега: все проверки здесь — чистые функции
// над строками, и их должен быть видно в тестах на любой платформе, а не
// только в линуксовом CI.
package cmd

import (
	"fmt"
	"strings"
)

var (
	repoURL          = "https://raw.githubusercontent.com/animesao/cardinal"
	releaseURL       = "https://github.com/animesao/cardinal"
	blueprintRepoURL = "https://raw.githubusercontent.com/cardinal-organization/cardinal-blueprints"
)

// releaseManifestURL — адрес подписываемого манифеста релиза.
//
// Подписывается SHA256SUMS.txt: манифест покрывает все артефакты релиза
// сразу, и подписывать надо ровно его. Раньше проверка искала подпись рядом
// с per-file чексуммами (cardinal_<ver>_checksums.txt.sig), которых в релизе
// нет, поэтому пройти она не могла ни при каких условиях.
func releaseManifestURL(releaseTag string) string {
	return fmt.Sprintf("%s/releases/download/%s/SHA256SUMS.txt", releaseURL, releaseTag)
}

// cosignIdentityRegexp строит regexp идентичности сертификата OIDC.
//
// Подпись делает релизный воркфлоу build.yml — идентичность должна
// указывать именно на него и быть привязана к тегу релиза. Тогда украденный
// ключ сам по себе не подходит: подпись должна быть сделана конкретным
// запуском этого воркфлоу на конкретном теге.
//
// Раньше здесь был воркфлоу release.yml, которого в репозитории нет, —
// проверка не могла пройти ни при каких условиях.
func cosignIdentityRegexp(releaseTag string) string {
	return fmt.Sprintf(`^https://github\.com/animesao/cardinal/\.github/workflows/build\.yml@refs/tags/%s$`,
		regexpQuoteMeta(releaseTag))
}

// regexpQuoteMeta is a tiny dependency-free implementation of regexp.QuoteMeta
// so cmd/update.go does not have to import the regexp package for one symbol.
func regexpQuoteMeta(s string) string {
	const special = `\.+*?()|[]{}^$`
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
