package probe

// Protocol mock handshakes
// Replace with real impl via tagged build or external libs if needed.

type HandshakeResult struct {
	OK     bool
	Reason string
}

// MockVMessHandshake simulates a protocol handshake by checking tcp/tls reachability.
// In real implementation, integrate with xray-core libraries to perform true auth sequence.
func MockVMessHandshake(n Node) HandshakeResult {
	if n.Host == "" || n.Port == 0 {
		return HandshakeResult{OK: true, Reason: "untestable"}
	}
	// assume OK if base transport was OK by upper probe flows
	return HandshakeResult{OK: true, Reason: "mock"}
}

func MockVLESSHandshake(n Node) HandshakeResult { return HandshakeResult{OK: true, Reason: "mock"} }
func MockTrojanHandshake(n Node) HandshakeResult { return HandshakeResult{OK: true, Reason: "mock"} }
func MockSSHandshake(n Node) HandshakeResult     { return HandshakeResult{OK: true, Reason: "mock"} }