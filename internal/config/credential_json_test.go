package config

import (
	"encoding/json"
	"testing"
)

func TestCredentialJSONCanonicalizesKnownFields(t *testing.T) {
	for _, input := range []string{
		`{"profiles":{"work":{"token":"fixture","authorization":null,"storage":null,"future":{"Token":"kept"}}},"future_root":true}`,
		`{"Profiles":{"work":{"Token":"fixture","Authorization":null,"Storage":null,"future":{"Token":"kept"}}},"future_root":true}`,
		`{"profileſ":{"work":{"toKen":"fixture","AUTHORIZATION":null,"ſtorage":null,"future":{"Token":"kept"}}},"future_root":true}`,
		`{"\u0050rofiles":{"work":{"To\u006ben":"fixture","Authorizatio\u006e":null,"Storag\u0065":null,"future":{"Token":"kept"}}},"future_root":true}`,
		`{"Profiles":{"work":{"token":"old"}},"profiles":{"work":{"Token":"old","token":"fixture","Authorization":{},"authorization":null,"Storage":{},"storage":null,"future":{"Token":"kept"}}},"future_root":true}`,
	} {
		var all Credentials
		if err := json.Unmarshal([]byte(input), &all); err != nil {
			t.Fatal(err)
		}
		c := all.Profiles["work"]
		if c.Token != "fixture" || string(c.Authorization) != "null" || string(c.Storage) != "null" {
			t.Fatal("known fields changed decoding semantics")
		}
		c.Token = ""
		c.Authorization = json.RawMessage(`{"version":1}`)
		c.Storage = json.RawMessage(`{"backend":"keychain"}`)
		all.Profiles["work"] = c
		data, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		var saved struct {
			Profiles map[string]map[string]json.RawMessage `json:"profiles"`
			Future   bool                                  `json:"future_root"`
		}
		var root map[string]json.RawMessage
		if json.Unmarshal(data, &saved) != nil || json.Unmarshal(data, &root) != nil {
			t.Fatal("cannot decode saved credentials")
		}
		fields := saved.Profiles["work"]
		if len(root) != 2 || !saved.Future || len(fields) != 3 || string(fields["future"]) != `{"Token":"kept"}` || string(fields["authorization"]) != `{"version":1}` || string(fields["storage"]) != `{"backend":"keychain"}` {
			t.Fatal("retained a known-field alias or lost unknown metadata")
		}
	}
}
