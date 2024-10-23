package sidecartemplates

import (
	"fmt"
	"log"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/docker/reference"
)

type VersionedTemplates[S, T any] struct {
	versions      semver.Collection
	templateFuncs map[semver.Version]TemplateFunc[S, T]
}

// GetFunc extracts the version tag from the image, parses it as semver and then
// finds the closest matching template function for it.
func (v VersionedTemplates[S, T]) GetFunc(dockerImage string) (TemplateFunc[S, T], error) {
	version, err := extractVersion(dockerImage)
	if err != nil {
		return nil, err
	}
	for _, minInclusiveVerion := range v.versions {
		if version.GreaterThanEqual(minInclusiveVerion) {
			return v.templateFuncs[*minInclusiveVerion], nil
		}
	}
	if len(v.templateFuncs) == 0 {
		log.Fatalln("there are no templates at all stored")
	}
	return nil, TemplateNotFoundErr
}

// LatestFunc returns the template function for the latest (most recent) version
func (v VersionedTemplates[S, T]) LatestFunc() TemplateFunc[S, T] {
	if len(v.templateFuncs) == 0 {
		log.Fatalln("there are no templates at all stored")
	}
	return v.templateFuncs[*v.versions[0]]
}

func extractVersion(image string) (*semver.Version, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, fmt.Errorf("cannot parse image")
	}
	ref = reference.TagNameOnly(ref)
	tagged, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("image does not containe a tag")
	}
	return semver.NewVersion(tagged.Tag())
}

type VersionedTemplate[S, T any] struct {
	MinInclusiveVersion semver.Version
	TemplateFunc        TemplateFunc[S, T]
}

func NewVersionedTemplates[S, T any](templates ...VersionedTemplate[S, T]) VersionedTemplates[S, T] {
	if len(templates) == 0 {
		log.Fatalln("cannot initialize a set of templates without any templates provided")
	}
	versions := semver.Collection{}
	templateFuncs := map[semver.Version]TemplateFunc[S, T]{}
	for i := range templates {
		template := &templates[i]
		versions = append(versions, &template.MinInclusiveVersion)
		templateFuncs[template.MinInclusiveVersion] = template.TemplateFunc
	}
	slices.SortFunc(versions, func(a, b *semver.Version) int {
		return b.Compare(a)
	})
	return VersionedTemplates[S, T]{
		versions:      versions,
		templateFuncs: templateFuncs,
	}
}

var TemplateNotFoundErr = fmt.Errorf("the template could not be found")

type TemplateFunc[S, T any] func(input *S) T
