package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Find dynamic map key names passed with Parent["foo"] notation
var bracketsRe = regexp.MustCompile(`\["([^\["\]]*)"\]`)

// Normalization for supporting arbitrary dynamic keys with dots:
// Gateway.PublicGateways["gw.example.com"].UseSubdomains
// Pinning.RemoteServices["pins.example.org"].Policies.MFS.Enable
func ConfigKeyToLookupData(key string) (normalizedKey string, dynamicKeys map[string]string) {
	bracketedKeys := bracketsRe.FindAllString(key, -1)
	dynamicKeys = make(map[string]string, len(bracketedKeys))
	normalizedKey = key
	for _, mapKeySegment := range bracketedKeys {
		mapKey := strings.TrimPrefix(mapKeySegment, `["`)
		mapKey = strings.TrimSuffix(mapKey, `"]`)
		placeholder := uuid.New().String()
		dynamicKeys[placeholder] = mapKey
		normalizedKey = strings.Replace(normalizedKey, mapKeySegment, fmt.Sprintf(".%s", placeholder), 1)
	}
	return normalizedKey, dynamicKeys
}

// Produces a part of config key with original map key names.
// Used only for better UX in error messages.
func buildSubKey(i int, parts []string, dynamicKeys map[string]string) string {
	subkey := strings.Join(parts[:i], ".")
	for placeholder, realKey := range dynamicKeys {
		subkey = strings.Replace(subkey, fmt.Sprintf(".%s", placeholder), fmt.Sprintf(`["%s"]`, realKey), 1)
	}
	return subkey
}

func MapGetKV(v map[string]interface{}, key string) (interface{}, error) {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	normalizedKey, dynamicKeys := ConfigKeyToLookupData(key)
	parts := strings.Split(normalizedKey, ".")
	for i, part := range parts {
		sofar := buildSubKey(i, parts, dynamicKeys)

		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s key is not a map", sofar)
		}
		if dynamicPart, ok := dynamicKeys[part]; ok {
			part = dynamicPart
		}
		cursor, ok = mcursor[part]
		if !ok {
			return nil, fmt.Errorf("%s key has no attribute %s", sofar, part)
		}
	}
	return cursor, nil
}

func MapSetKV(v map[string]interface{}, key string, value interface{}) error {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	normalizedKey, dynamicKeys := ConfigKeyToLookupData(key)
	parts := strings.Split(normalizedKey, ".")
	for i, part := range parts {
		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			sofar := buildSubKey(i, parts, dynamicKeys)
			return fmt.Errorf("%s key is not a map", sofar)
		}
		if dynamicPart, ok := dynamicKeys[part]; ok {
			part = dynamicPart
		}

		// last part? set here
		if i == (len(parts) - 1) {
			mcursor[part] = value
			break
		}

		cursor, ok = mcursor[part]
		if !ok || cursor == nil { // create map if this is empty or is null
			mcursor[part] = map[string]interface{}{}
			cursor = mcursor[part]
		}
	}
	return nil
}
