//go:build !darwin

package store

// defaultDomains uses the files in ~/.quosparr. There is no legacy domain.
func defaultDomains() (current, legacy Domain, err error) {
	dir, err := DataDir()
	if err != nil {
		return nil, nil, err
	}
	return FileDomain{Dir: dir}, nil, nil
}
