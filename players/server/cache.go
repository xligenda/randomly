package server

import (
	"fmt"
	"players/mc"
	"time"
)

func (s *PlayerServer) getStatusWithCache(address string) (*mc.StatusResponse, error) {
	s.mu.RLock()
	isCacheValid := s.lastStatus != nil && time.Since(s.lastPingedAt) < cacheDuration
	if isCacheValid {
		defer s.mu.RUnlock()
		return s.lastStatus, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastStatus != nil && time.Since(s.lastPingedAt) < cacheDuration {
		return s.lastStatus, nil
	}

	statusResp, err := mc.Ping(address, s.Timeout)
	if err != nil {
		if s.lastStatus != nil {
			return s.lastStatus, nil
		}
		return nil, fmt.Errorf("failed to ping %q: %w", address, err)
	}

	s.lastStatus = statusResp
	s.lastPingedAt = time.Now()

	return statusResp, nil
}
