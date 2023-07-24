// SPDX-FileCopyrightText: 2023 Christoph Mewes
// SPDX-License-Identifier: MIT

package gimps

import (
	"testing"
)

func TestIsProjectImport(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		importPath  string
		expected    bool
	}{
		{
			name:        "same path",
			projectName: "github.com/foo/bar",
			importPath:  "github.com/foo/bar",
			expected:    true,
		},
		{
			name:        "regular sub package",
			projectName: "github.com/foo/bar",
			importPath:  "github.com/foo/bar/subpkg",
			expected:    true,
		},
		{
			name:        "name collision",
			projectName: "github.com/foo/bar",
			importPath:  "github.com/foo/bartiocelli",
			expected:    false,
		},
		{
			name:        "different kind of collision",
			projectName: "github.com/foo/bar",
			importPath:  "github.com/foo/bar-tiocelli",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier := NewClassifier(tt.projectName, nil)

			result := classifier.IsProjectImport(tt.importPath)
			if result != tt.expected {
				t.Errorf("isProjectImport() returned %v, but wanted %v", result, tt.expected)
			}
		})
	}
}
