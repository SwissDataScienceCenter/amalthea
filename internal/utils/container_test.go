/*
Copyright 2025.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnrootImageFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		out string
	}{
		// Tests on Docker library images
		{
			in:  "python:3.13@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
			out: "python@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
		},
		{
			in:  "python:3.13",
			out: "python:3.13",
		},
		{
			in:  "python@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
			out: "python@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
		},
		{
			in:  "python",
			out: "python:latest",
		},
		// Tests on images hosted on Docker Hub
		{
			in:  "renku/amalthea-sessions:0.21.0@sha256:e0be19853aa5359039ea6ec2a2277b8dc4f404f14de5645e0cb604426b326ee3",
			out: "renku/amalthea-sessions@sha256:e0be19853aa5359039ea6ec2a2277b8dc4f404f14de5645e0cb604426b326ee3",
		},
		{
			in:  "renku/amalthea-sessions:0.21.0",
			out: "renku/amalthea-sessions:0.21.0",
		},
		{
			in:  "renku/amalthea-sessions@sha256:e0be19853aa5359039ea6ec2a2277b8dc4f404f14de5645e0cb604426b326ee3",
			out: "renku/amalthea-sessions@sha256:e0be19853aa5359039ea6ec2a2277b8dc4f404f14de5645e0cb604426b326ee3",
		},
		{
			in:  "renku/amalthea-sessions",
			out: "renku/amalthea-sessions:latest",
		},
		// Tests on images hosted in other registries
		{
			in:  "harbor.dev.renku.ch/renku-build/renku-build:renku-01k604nerkh0x9qjehbwmr8vyf",
			out: "harbor.dev.renku.ch#renku-build/renku-build:renku-01k604nerkh0x9qjehbwmr8vyf",
		},
		{
			in:  "harbor.dev.renku.ch/deeply/nested/path:main",
			out: "harbor.dev.renku.ch#deeply/nested/path:main",
		},
		{
			in:  "harbor.dev.renku.ch/renku-build/renku-build:renku-01k604nerkh0x9qjehbwmr8vyf@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
			out: "harbor.dev.renku.ch#renku-build/renku-build@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
		},
		{
			in:  "harbor.dev.renku.ch/renku-build/renku-build@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
			out: "harbor.dev.renku.ch#renku-build/renku-build@sha256:2deb0891ec3f643b1d342f04cc22154e6b6a76b41044791b537093fae00b6884",
		},
		{
			in:  "harbor.dev.renku.ch/renku-build/renku-build",
			out: "harbor.dev.renku.ch#renku-build/renku-build:latest",
		},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()
			t.Log(test.in)

			result, err := EnrootImageFormat(test.in)
			assert.NoError(t, err)
			assert.Equal(t, test.out, result)
		})
	}
}
