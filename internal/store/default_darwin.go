//go:build darwin

package store

// defaultDomains uses the Swift app's UserDefaults domains.
func defaultDomains() (current, legacy Domain, err error) {
	current = NewUserDefaultsDomain(DomainName)
	return current, NewUserDefaultsDomain(LegacyDomainName), nil
}
