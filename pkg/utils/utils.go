package utils

import "gopkg.in/yaml.v3"

func StringInList(s string, list []string) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}

	return false
}

func YamlMarshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func YamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
