// Copyright 2026 JOSE MARIA BECERRA VAZQUEZ
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

import "testing"

func TestMaskToken(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "cfx...[empty]"},
		{"abc", "cfx...abc"},
		{"abcd", "cfx...abcd"},
		{"abcde", "cfx...bcde"},
		{"abc1234567890abcdef1234", "cfx...1234"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := MaskToken(c.input)
			if got != c.want {
				t.Errorf("MaskToken(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}
