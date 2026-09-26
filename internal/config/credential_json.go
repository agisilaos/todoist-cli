package config

import "encoding/json"

// Preserve fields owned by newer versions when modifying a different profile.
func (c *Credential) UnmarshalJSON(data []byte) error {
	type known Credential
	var value known
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = Credential(value)
	if err := json.Unmarshal(data, &c.extra); err != nil {
		return err
	}
	delete(c.extra, "token")
	delete(c.extra, "authorization")
	return nil
}

func (c Credential) MarshalJSON() ([]byte, error) {
	fields := copyJSONFields(c.extra)
	token, _ := json.Marshal(c.Token)
	fields["token"] = token
	if c.Authorization != nil {
		fields["authorization"] = c.Authorization
	}
	return json.Marshal(fields)
}

func (c *Credentials) UnmarshalJSON(data []byte) error {
	type known Credentials
	var value known
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = Credentials(value)
	if err := json.Unmarshal(data, &c.extra); err != nil {
		return err
	}
	delete(c.extra, "profiles")
	return nil
}

func (c Credentials) MarshalJSON() ([]byte, error) {
	fields := copyJSONFields(c.extra)
	profiles, err := json.Marshal(c.Profiles)
	if err != nil {
		return nil, err
	}
	fields["profiles"] = profiles
	return json.Marshal(fields)
}

func copyJSONFields(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in)+2)
	for k, v := range in {
		out[k] = v
	}
	return out
}
