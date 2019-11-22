package cab_test

import (
	"testing"

	"github.com/craiggwilson/go-cab/pkg/cab"
)

func TestReader(t *testing.T) {
	testCases := []struct {
		path      string
		fileNames [][]string
	}{
		{
			path: "testdata/readme.cab",
			fileNames: [][]string{
				[]string{"README.md"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			r, err := cab.OpenReader(tc.path)
			if err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}

			if len(r.Folders) != len(tc.fileNames) {
				t.Fatalf("expected %d folder(s), but got %d", len(tc.fileNames), len(r.Folders))
			}

			for i, folder := range r.Folders {
				for _, file := range folder.Files {
					if !stringSliceContains(tc.fileNames[i], file.Name) {
						t.Fatalf("did not find %q in the expected file names", file.Name)
					}
				}
			}
			defer r.Close()
		})
	}
}

func stringSliceContains(slice []string, s string) bool {
	for _, i := range slice {
		if i == s {
			return true
		}
	}

	return false
}
