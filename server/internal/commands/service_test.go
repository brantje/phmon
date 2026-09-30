package commands

import "testing"

type versionedCapabilityFixture struct{ protocol int }

func (v versionedCapabilityFixture) CommandSupport(string, uint64, string) (bool, string) {
	return false, "plugin_upgrade_required"
}
func (v versionedCapabilityFixture) ProtocolVersion(string, uint64) int { return v.protocol }

func TestControlSnapshotCarriesNegotiatedAgentProtocolVersion(t *testing.T) {
	for _, version := range []int{7, 8} {
		service := NewService(nil, versionedCapabilityFixture{protocol: version})
		snapshot := service.controlSnapshot(Target{
			CharacterID: "character-one", SessionID: "session-one", AgentID: "agent-one", Generation: 3,
		}, nil)
		if got := snapshot["agent_protocol_version"]; got != version {
			t.Fatalf("protocol version = %#v, want %d", got, version)
		}
	}
}
