/*
Copyright 2024.

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

// Based on: https://github.com/spf13/pflag/issues/236#issuecomment-931600452

package cmd

import (
	"fmt"
	"slices"
	"strings"
)

type enum struct {
	Allowed []string
	Value   string
}

// newEnum give a list of allowed flag parameters, where the second argument is the default
func newEnum(allowed []string, def string) *enum {
	return &enum{
		Allowed: allowed,
		Value:   def,
	}
}

func (flag enum) String() string {
	return flag.Value
}

func (flag *enum) Set(value string) error {
	if !slices.Contains(flag.Allowed, value) {
		return fmt.Errorf("%s is not included in %s", value, strings.Join(flag.Allowed, ","))
	}
	flag.Value = value
	return nil
}

func (flag *enum) Type() string {
	return "string"
}

func (flag *enum) Equal(value string) bool {
	return flag.Value == value
}
