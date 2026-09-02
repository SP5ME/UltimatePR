package config

import (
	"fmt"
	"strings"
)

func validateServiceIDs(c Config) error {
	serviceIDs := map[string]string{}
	for label, value := range map[string]struct {
		id      string
		enabled bool
	}{
		"node.service_id":      {id: c.Node.ServiceID, enabled: c.Node.Enabled},
		"bbs.service_id":       {id: c.BBS.ServiceID, enabled: c.BBS.Enabled},
		"ai.service_id":        {id: c.AI.ServiceID, enabled: c.AI.Enabled},
		"game_hall.service_id": {id: c.GameHall.ServiceID, enabled: c.GameHall.Enabled},
	} {
		id := strings.ToLower(strings.TrimSpace(value.id))
		if id == "" {
			continue
		}
		if owner, ok := serviceIDs[id]; ok {
			return fmt.Errorf("%s duplicates %s", label, owner)
		}
		serviceIDs[id] = label
	}
	return nil
}
