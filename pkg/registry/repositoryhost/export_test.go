package repositoryhost

var NewResourceURL = new

// ResetHosts restores knownHosts to its default state between tests.
func ResetHosts() {
	knownHosts = []string{"github.com"}
	cachedRaw, cachedResource = buildRegexps()
}
