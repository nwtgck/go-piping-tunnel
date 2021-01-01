package piping_tunnel_util

import (
	"github.com/pkg/errors"
	"strings"
)

type KeyValue struct {
	Key   string
	Value string
}

func ParseKeyValueStrings(strKeyValues []string) ([]KeyValue, error) {
	var keyValues []KeyValue
	for _, str := range strKeyValues {
		splitted := strings.SplitN(str, ":", 2)
		if len(splitted) != 2 {
			return nil, errors.Errorf("invalid header format '%s'", str)
		}
		keyValues = append(keyValues, KeyValue{Key: splitted[0], Value: splitted[1]})
	}
	return keyValues, nil
}
