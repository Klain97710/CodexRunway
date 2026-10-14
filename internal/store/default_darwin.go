//go:build darwin

package store

// defaultDomains uses the Swift app's UserDefaults domains.
func defaultDomains() (current, legacy Domain, err error) {
	current = renamedDomain{NewUserDefaultsDomain(DomainName), []Domain{
		NewUserDefaultsDomain(PreviewDomainName),
		NewUserDefaultsDomain(PreviousDomainName),
	}}
	return current, NewUserDefaultsDomain(LegacyDomainName), nil
}
