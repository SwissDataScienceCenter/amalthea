package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/distribution/reference"
)

// ImageFieldRewriter rewrites the "image" field for sessions
type ImageFieldRewriter interface {
	// Rewrite returns a new value for the "image" field
	Rewrite(image string) (newImage string, err error)
}

// GetImageFieldRewriter returns the image rewriter configured by the env var "AMALTHEA_IMAGE_REWRITE_RULES"
func GetImageFieldRewriter() (rewriter ImageFieldRewriter, err error) {
	rulesStr := os.Getenv("AMALTHEA_IMAGE_REWRITE_RULES")
	if rulesStr == "" {
		return nil, nil
	}
	rules, err := parseImageRewriteRules(rulesStr)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	return &ruleBasedRewriter{rules}, nil
}

func parseImageRewriteRules(rulesStr string) (rules []imageRewriteRule, err error) {
	if err := json.Unmarshal([]byte(rulesStr), &rules); err != nil {
		return []imageRewriteRule{}, err
	}
	return rules, nil
}

type imageRewriteRule struct {
	SourcePrefix string `json:"source_prefix"`
	TargetPrefix string `json:"target_prefix"`
}

// rewrite the domain and path part of a container image reference according to the rule
func (rule *imageRewriteRule) rewrite(domainAndPath string) (result string, match bool) {
	after, found := strings.CutPrefix(domainAndPath, rule.SourcePrefix)
	if !found || !strings.HasPrefix(after, "/") {
		return domainAndPath, false
	}
	return rule.TargetPrefix + after, true
}

type ruleBasedRewriter struct {
	rules []imageRewriteRule
}

func (r *ruleBasedRewriter) Rewrite(image string) (newImage string, err error) {
	named, err := reference.ParseDockerRef(image)
	if err != nil {
		return image, err
	}
	domain := reference.Domain(named)
	path := reference.Path(named)
	domainAndPath := fmt.Sprintf("%s/%s", domain, path)
	tagged, isTagged := named.(reference.Tagged)
	digested, isDigested := named.(reference.Digested)
	for _, rule := range r.rules {
		// Skip malformed rules
		if rule.SourcePrefix == "" || rule.TargetPrefix == "" {
			continue
		}
		result, match := rule.rewrite(domainAndPath)
		if match {
			if isTagged && isDigested {
				tag := tagged.Tag()
				digest := digested.Digest()
				return fmt.Sprintf("%s:%s@%s", result, tag, digest), nil
			}
			if isTagged {
				tag := tagged.Tag()
				return fmt.Sprintf("%s:%s", result, tag), nil
			}
			if isDigested {
				digest := digested.Digest()
				return fmt.Sprintf("%s@%s", result, digest), nil
			}
			return domainAndPath, nil
		}
	}
	return image, nil
}
