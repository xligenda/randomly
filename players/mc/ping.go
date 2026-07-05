package mc

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Ping connects to the server over TCP, performs a handshake followed
// by a status request, and returns the server's parsed status.
func Ping(address string, timeout time.Duration) (*StatusResponse, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q (expected host:port): %w", address, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: %w", portStr, err)
	}

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := writeHandshake(conn, host, uint16(port)); err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	if err := writeStatusRequest(conn); err != nil {
		return nil, fmt.Errorf("status request: %w", err)
	}

	status, err := readStatusResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("reading status: %w", err)
	}
	return status, nil
}
