package resources

func (m *ItemMetadata) ReverseReturnLocations(server string) []string {
	dataset, ok := m.DatasetForServer(server)
	if !ok {
		return nil
	}
	return append([]string(nil), m.ReverseReturnNames[dataset]...)
}
