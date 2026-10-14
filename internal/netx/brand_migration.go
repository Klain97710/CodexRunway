package netx

import "log"

// renamedSecrets copies a referenced proxy credential into the new service on
// first use. It never removes the original or bypasses an OS permission error.
type renamedSecrets struct {
	current  rawSecrets
	previous []rawSecrets
}

func (s renamedSecrets) get(id string, allow bool) ([]byte, bool, error) {
	data, found, err := s.current.get(id, allow)
	if err != nil || found {
		return data, found, err
	}
	for _, previous := range s.previous {
		data, found, err = previous.get(id, allow)
		if err != nil || found {
			break
		}
	}
	if err != nil || !found {
		return data, found, err
	}
	if _, err := decodeCredentials(data); err != nil {
		return nil, true, err
	}
	if err := s.current.set(id, data); err != nil {
		return nil, true, err
	}
	log.Println("migrated saved proxy credentials to Quosparr")
	return data, true, nil
}

func (s renamedSecrets) set(id string, data []byte) error { return s.current.set(id, data) }
func (s renamedSecrets) del(id string) error              { return s.current.del(id) }
