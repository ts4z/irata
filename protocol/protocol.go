// package protocol defines constants and types related to the client-server protocol.package protocol

package protocol

const (
	// Version indicates an incompatible change to the client/server interaction.
	// If the client gets a different number than it originally got here, it should reload
	// to get a new copy of all server files.  This does not indicate any particular
	// compatibility problem.
	Version = 13
)
