package config

import (
	"fmt"
	"net"
	"strings"
)

func validateRemoteEndpoints(c Config) error {
	ids := map[string]bool{}
	for _, id := range []string{c.Node.ServiceID, c.BBS.ServiceID, c.AI.ServiceID, c.GameHall.ServiceID} {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			ids[id] = true
		}
	}
	addresses := map[string]string{}
	local := map[string]string{}
	addLocal := func(address, owner string) error {
		if previous := local[address]; previous != "" {
			return fmt.Errorf("local callsign %q conflicts with %s and %s service", address, previous, owner)
		}
		local[address] = owner
		return nil
	}
	if c.BBS.Enabled {
		if err := addLocal(strings.ToUpper(c.BBS.Callsign)+fmt.Sprintf("-%d", c.BBS.SSID), "bbs"); err != nil {
			return err
		}
	}
	if c.Experimental.Services && c.AI.Enabled {
		if err := addLocal(strings.ToUpper(c.AI.Callsign)+fmt.Sprintf("-%d", c.AI.SSID), "ai"); err != nil {
			return err
		}
	}
	if c.Experimental.Services && c.GameHall.Enabled {
		if err := addLocal(strings.ToUpper(c.GameHall.Callsign)+fmt.Sprintf("-%d", c.GameHall.SSID), "game_hall"); err != nil {
			return err
		}
	}
	for i, endpoint := range c.RemoteEndpoints {
		id := strings.ToLower(strings.TrimSpace(endpoint.ID))
		if id == "" || ids[id] {
			return fmt.Errorf("remote_endpoints[%d]: id is empty or duplicated", i)
		}
		ids[id] = true
		if endpoint.Enabled {
			call, err := parseStationText(endpoint.Callsign)
			if err != nil {
				return fmt.Errorf("remote_endpoints[%d].callsign: %w", i, err)
			}
			address := strings.ToUpper(call.Callsign) + fmt.Sprintf("-%d", call.SSID)
			if owner := local[address]; owner != "" {
				return fmt.Errorf("remote_endpoints[%d]: callsign %q conflicts with local %s service", i, address, owner)
			}
			if owner := addresses[address]; owner != "" {
				return fmt.Errorf("remote_endpoints[%d]: callsign %q conflicts with remote endpoint %q", i, address, owner)
			}
			addresses[address] = id
		}
		transport := strings.ToLower(strings.TrimSpace(endpoint.Transport))
		if transport != "ax25" && transport != "node" && transport != "tcp" {
			return fmt.Errorf("remote_endpoints[%d].transport must be ax25, node or tcp", i)
		}
		if transport == "tcp" && (strings.TrimSpace(endpoint.Host) == "" || endpoint.Port == 0) {
			return fmt.Errorf("remote_endpoints[%d]: tcp host and port are required", i)
		}
		if transport != "tcp" && endpoint.Port != 0 && endpoint.Port > 65535 {
			return fmt.Errorf("remote_endpoints[%d].port is invalid", i)
		}
		if endpoint.Host != "" && transport == "tcp" {
			if _, _, err := net.SplitHostPort(net.JoinHostPort(endpoint.Host, fmt.Sprint(endpoint.Port))); err != nil {
				return fmt.Errorf("remote_endpoints[%d]: invalid tcp address: %w", i, err)
			}
		}
	}
	return nil
}

func validateForwardingRFPorts(c Config) error {
	ports := map[string]bool{}
	for _, p := range c.Ports {
		if p.Enabled != nil && !*p.Enabled {
			continue
		}
		ports[p.ID] = p.Type == "kiss-tcp" || p.Type == "axudp"
		for _, logicalID := range p.Channels {
			ports[logicalID] = true
		}
	}
	validate := func(path, rfPort string, fallback bool) error {
		if !fallback {
			return nil
		}
		rfPort = strings.TrimSpace(rfPort)
		if rfPort == "" {
			return fmt.Errorf("%s: rf_port is required when fallback_to_rf is enabled", path)
		}
		if capable, ok := ports[rfPort]; !ok || !capable {
			return fmt.Errorf("%s: rf_port %q does not exist, is disabled, or is not AX.25-capable", path, rfPort)
		}
		return nil
	}
	for i, endpoint := range c.RemoteEndpoints {
		if err := validate(fmt.Sprintf("remote_endpoints[%d]", i), endpoint.RFPort, endpoint.FallbackToRF); err != nil {
			return err
		}
	}
	for i, peer := range c.BBS.Forwarding.Peers {
		rfPort := peer.RFPort
		fallback := peer.FallbackToRF
		if strings.TrimSpace(peer.EndpointID) != "" {
			for _, endpoint := range c.RemoteEndpoints {
				if strings.EqualFold(endpoint.ID, peer.EndpointID) {
					rfPort, fallback = endpoint.RFPort, endpoint.FallbackToRF
					break
				}
			}
		}
		if err := validate(fmt.Sprintf("bbs.forwarding.peers[%d]", i), rfPort, fallback); err != nil {
			return err
		}
	}
	return nil
}
