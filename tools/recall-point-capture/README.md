# Manual recall-point packet capture

This is a temporary **read-only** phBot plugin for the Designate Recall Point evidence gate. It does not send packets, operate a character, write files or contact PhMon. The operator chooses a test character and performs one manual designation.

1. Copy `RecallPointCapture.py` into that phBot installation's `Plugins` directory. A shared installation may show the tab in several sessions; arm it only in the chosen test session.
2. Stand at a known teleporter. In the capture tab, select **Arm 15-second capture**. The log records the nearby `get_npcs()` gate IDs.
3. Immediately perform the normal **Designate Recall Point** action once, then wait for `RecallCapture complete` in the phBot log.
4. Provide only the `RecallCapture` log lines and the observed in-game result. Remove the temporary plugin afterward.

The probe logs candidate outgoing `0x7045` / `0x7059` payloads up to 16 bytes, the one-byte `0xB059` response payload, other outgoing `0x70xx` opcodes and lengths, and bounded incoming opcode/length metadata. It never logs other packet payloads. The official `handle_silkroad` callback receives packets from the **game client**; a designation initiated inside phBot's own map might bypass this callback. If no outgoing candidate appears, that is an inconclusive capture, not evidence that the packet is absent. In that case, repeat only after choosing an appropriate capture path; do not infer or send `0x7059` from this tool alone.
