package firecrest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchRegExp(t *testing.T) {
	line := "[branch \"main\"]"
	line = strings.TrimSpace(line)
	res := branchRegExp.FindStringSubmatch(line)
	assert.Len(t, res, 2)
	assert.Equal(t, "[branch \"main\"]", res[0])
	assert.Equal(t, "main", res[1])
}
