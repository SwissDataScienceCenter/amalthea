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
	"fmt"

	"github.com/distribution/reference"
)

func EnrootImageFormat(dockerImage string) (enrootImage string, err error) {
	named, err := reference.ParseDockerRef(dockerImage)
	if err != nil {
		return enrootImage, err
	}
	domain := reference.Domain(named)
	// If domain is "docker.io", we just return the familiar string without any "#" character
	if domain == "docker.io" {
		return reference.FamiliarString(named), nil
	}
	// Construct the image string including the "#" character
	path := reference.Path(named)
	tagged, isTagged := named.(reference.Tagged)
	digested, isDigested := named.(reference.Digested)
	if isTagged && isDigested {
		tag := tagged.Tag()
		digest := digested.Digest()
		return fmt.Sprintf("%s#%s:%s@%s", domain, path, tag, digest), nil
	}
	if isTagged {
		tag := tagged.Tag()
		return fmt.Sprintf("%s#%s:%s", domain, path, tag), nil
	}
	if isDigested {
		digest := digested.Digest()
		return fmt.Sprintf("%s#%s@%s", domain, path, digest), nil
	}
	return fmt.Sprintf("%s#%s", domain, path), nil
}
