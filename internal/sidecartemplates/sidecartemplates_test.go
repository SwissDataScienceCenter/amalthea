package sidecartemplates

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testResource struct {
	name string
}
type testManifests struct {
	name string
}

// Note the test includes pre-releases but it is better to avoid these in real usage
// because sorting prerlease version can get tricky.
func makeNewTemplates() VersionedTemplates[testResource, testManifests] {
	return NewVersionedTemplates(
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("v0.0.4"),
			func(input *testResource) testManifests {
				return testManifests{"v0.0.4"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.0.3-alpha"),
			func(input *testResource) testManifests {
				return testManifests{"0.0.3-alpha"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.0.3-aa1"),
			func(input *testResource) testManifests {
				return testManifests{"0.0.3-aa1"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.0.10-alpha.1"),
			func(input *testResource) testManifests {
				return testManifests{"0.0.10-alpha.1"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.0.10-beta"),
			func(input *testResource) testManifests {
				return testManifests{"0.0.10-beta"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.1.0"),
			func(input *testResource) testManifests {
				return testManifests{"0.1.0"}
			},
		},
		VersionedTemplate[testResource, testManifests]{
			*semver.MustParse("0.1.0-beta"),
			func(input *testResource) testManifests {
				return testManifests{"0.1.0-beta"}
			},
		},
	)
}

func TestGetFunc(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	templs := makeNewTemplates()
	// You cannot get a template for an image whose semver is older than the oldest versioned template
	_, err := templs.GetFunc("renku/sidecars:0.0.1")
	assert.ErrorIs(err, TemplateNotFoundErr)
	// The returned image should have a semver older or equal to the requested
	f, err := templs.GetFunc("renku/sidecars:0.0.5")
	require.NoError(err)
	res := f(&testResource{})
	assert.Equal(t, "v0.0.4", res.name)
	// Alpha versions are properly sorted
	f, err = templs.GetFunc("renku/sidecars:0.0.3")
	require.NoError(err)
	res = f(&testResource{})
	assert.Equal(t, "0.0.3-alpha", res.name)
	// Asking for a version that is newer than anything stored should return the newest
	f, err = templs.GetFunc("renku/sidecars:0.4.0")
	require.NoError(err)
	res = f(&testResource{})
	assert.Equal(t, "0.1.0", res.name)
	// Changing the image name does not matter - just the semver matters
	f, err = templs.GetFunc("image:0.4.0")
	require.NoError(err)
	res = f(&testResource{})
	assert.Equal(t, "0.1.0", res.name)
	// Ensure that prerelease versions are sorted correctly
	f, err = templs.GetFunc("image:0.0.10")
	require.NoError(err)
	res = f(&testResource{})
	assert.Equal(t, "0.0.10-beta", res.name)
}
