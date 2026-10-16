package resources

import "strconv"

// ZoneName resolves a region ID to its client zone name using the server's
// mapped game-data profile. It returns "" when the region is unknown.
func (m *ItemMetadata) ZoneName(server string, region int) string {
	if m == nil {
		return ""
	}
	dataset, ok := m.DatasetForServer(server)
	if !ok {
		return ""
	}
	if region > 32767 {
		region -= 65536
	}
	return m.ZoneNames[dataset][strconv.Itoa(region)]
}

func (s *Store) ZoneName(server string, region int) string {
	if s == nil {
		return ""
	}
	return s.metadata.ZoneName(server, region)
}
