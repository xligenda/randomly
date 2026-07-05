package mc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

const (
	packetHandshake      = 0x00
	packetStatusRequest  = 0x00
	packetStatusResponse = 0x00

	handshakeNextStateStatus = 1
	handshakeProtocolVersion = 767
	maxStatusJSONLen         = 1 << 20
)

func writeHandshake(conn net.Conn, host string, port uint16) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, packetHandshake)
	_ = writeVarInt(&body, handshakeProtocolVersion)
	_ = writeString(&body, host)
	_ = binary.Write(&body, binary.BigEndian, port)
	_ = writeVarInt(&body, handshakeNextStateStatus)

	return writePacket(conn, body.Bytes())
}

func writeStatusRequest(conn net.Conn) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, packetStatusRequest)
	return writePacket(conn, body.Bytes())
}

func writePacket(conn net.Conn, body []byte) error {
	var packet bytes.Buffer
	if err := writeVarInt(&packet, int32(len(body))); err != nil {
		return err
	}
	if _, err := packet.Write(body); err != nil {
		return err
	}
	_, err := conn.Write(packet.Bytes())
	return err
}

func readStatusResponse(conn net.Conn) (*StatusResponse, error) {
	r := bufio.NewReader(conn)

	if _, err := readVarInt(r); err != nil { // total packet length
		if err == io.EOF {
			return nil, fmt.Errorf("server closed the connection without sending any data "+
				"(likely an anti-bot/proxy protection blocking non-standard connections): %w", err)
		}
		return nil, err
	}

	pid, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	if pid != packetStatusResponse {
		return nil, fmt.Errorf("unexpected packet id: %d", pid)
	}

	strLen, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	if strLen < 0 || strLen > maxStatusJSONLen {
		return nil, fmt.Errorf("suspicious status JSON length: %d", strLen)
	}

	jsonBytes := make([]byte, strLen)
	if _, err := io.ReadFull(r, jsonBytes); err != nil {
		return nil, err
	}

	var status StatusResponse
	if err := json.Unmarshal(jsonBytes, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status JSON: %w", err)
	}
	return &status, nil
}
