package assert

import (
	"encoding/json"
	"strings"

	"github.com/stretchr/testify/assert"
)

func MatchesJSONMap(t assert.TestingT, object any, expectedJSON map[string]any) {
	expectedBytes, err := json.Marshal(expectedJSON)
	assert.NoError(t, err)

	MatchesJSONBytes(t, object, expectedBytes)
}

func MatchesJSONString(t assert.TestingT, object any, expectedJSON string) {
	MatchesJSONBytes(t, object, []byte(expectedJSON))
}

func MatchesJSONBytes(t assert.TestingT, object any, expectedJSON []byte) {
	objectBytes, err := json.Marshal(object)
	assert.NoError(t, err)

	// Use strings instead of []byte for reasonable error message
	assert.Equal(t, string(expectedJSON), string(objectBytes))
}

func LacksJSONKey(t assert.TestingT, object any, key string) {
	objectBytes, err := json.Marshal(object)
	assert.NoError(t, err)

	objectAsMap := make(map[string]any)

	err = json.Unmarshal(objectBytes, &objectAsMap)
	assert.NoError(t, err)

	_, found := objectAsMap[key]

	objectKeys := make([]string, len(objectAsMap))
	i := 0
	for k := range objectAsMap {
		objectKeys[i] = k
		i++
	}

	assert.False(t, found, `Expected object with keys "%s" to not have key "%s"`, strings.Join(objectKeys, ", "), key)
}
