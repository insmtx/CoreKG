package svccliauth

import "encoding/json"

func marshalSession(value Session) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func unmarshalSession(value string, target *Session) error {
	return json.Unmarshal([]byte(value), target)
}
