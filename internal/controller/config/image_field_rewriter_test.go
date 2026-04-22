package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageRewriteRuleRewrite(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input          string
		expectedMatch  bool
		extectedResult string
	}{
		{input: "node", expectedMatch: false},
		{input: "python:3.13", expectedMatch: false},
		{input: "example.org", expectedMatch: false},
		{input: "example.org/repo", expectedMatch: true, extectedResult: "my-cache.com/prefix/repo"},
		{input: "example.org/repo/image", expectedMatch: true, extectedResult: "my-cache.com/prefix/repo/image"},
		{input: "example.or/repo/image", expectedMatch: false},
		{input: "example.orgu/repo/image", expectedMatch: false},
	}

	rule := imageRewriteRule{
		SourcePrefix: "example.org",
		TargetPrefix: "my-cache.com/prefix",
	}

	for _, testCase := range cases {
		name := strings.ReplaceAll(testCase.input, "/", "_")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, match := rule.rewrite(testCase.input)
			if testCase.expectedMatch {
				assert.Truef(t, match, "expected rule to match")
				assert.Equal(t, testCase.extectedResult, result)
			} else {
				assert.Falsef(t, match, "expected rule to not match")
			}
		})
	}
}

func TestRuleBasedRewriter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input  string
		output string
	}{
		{input: "node", output: "my-cache.com/prefix/library/node:latest"},
		{input: "python:3.13", output: "my-cache.com/prefix/library/python:3.13"},
		{input: "renku/jupyterlab", output: "my-cache.com/prefix/renku/jupyterlab:latest"},
		{input: "renku/renkulab-r:4.3.1-0.25.0", output: "my-cache.com/prefix/renku/renkulab-r:4.3.1-0.25.0"},
		{input: "docker.io/node", output: "my-cache.com/prefix/library/node:latest"},
		{input: "docker.io/python:3.13", output: "my-cache.com/prefix/library/python:3.13"},
		{input: "docker.io/renku/jupyterlab", output: "my-cache.com/prefix/renku/jupyterlab:latest"},
		{input: "example.org/repo", output: "my-cache.com/prefix2/repo:latest"},
		{input: "example.org/repo/image", output: "my-cache.com/prefix2/repo/image:latest"},
		{input: "example.org/repo/image:tag", output: "my-cache.com/prefix2/repo/image:tag"},
		{input: "example.or/repo/image", output: "example.or/repo/image"},
		{input: "example.orgu/repo/image", output: "example.orgu/repo/image"},
	}

	var rewriter ImageFieldRewriter = &ruleBasedRewriter{
		rules: []imageRewriteRule{
			{
				SourcePrefix: "docker.io",
				TargetPrefix: "my-cache.com/prefix",
			},
			{
				SourcePrefix: "example.org",
				TargetPrefix: "my-cache.com/prefix2",
			},
		},
	}

	for _, testCase := range cases {
		name := strings.ReplaceAll(testCase.input, "/", "_")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output, err := rewriter.Rewrite(testCase.input)
			require.NoError(t, err)
			assert.Equal(t, testCase.output, output)
		})
	}
}

func TestParseImageRewriteRules(t *testing.T) {
	rulesStr := `[
		{
			"source_prefix": "docker.io",
			"target_prefix": "my-cache.com/prefix"
		},
		{
			"source_prefix": "example.org",
			"target_prefix": "my-cache.com/prefix2"
		}
	]`

	rules, err := parseImageRewriteRules(rulesStr)
	require.NoError(t, err)

	expectedRules := []imageRewriteRule{
		{
			SourcePrefix: "docker.io",
			TargetPrefix: "my-cache.com/prefix",
		},
		{
			SourcePrefix: "example.org",
			TargetPrefix: "my-cache.com/prefix2",
		},
	}
	assert.Equal(t, expectedRules, rules)
}
