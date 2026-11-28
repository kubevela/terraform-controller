/*
Copyright 2026 The KubeVela Authors.

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

package app

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressedTFStateIsGzipOfTheFile(t *testing.T) {
	state := []byte(`{"version": 4, "serial": 2, "resources": []}` + "\n")
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, state, 0o600); err != nil {
		t.Fatal(err)
	}

	compressed, err := compressedTFState(path)
	if err != nil {
		t.Fatalf("compressedTFState: %v", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("result is not gzip: %v", err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading the result: %v", err)
	}
	if !bytes.Equal(state, got) {
		t.Errorf("round trip changed the state: got %q, want %q", got, state)
	}
}

func TestCompressedTFStateOfAnEmptyFileIsValidGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	compressed, err := compressedTFState(path)
	if err != nil {
		t.Fatalf("compressedTFState: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("result is not gzip: %v", err)
	}
	if got, err := io.ReadAll(reader); err != nil || len(got) != 0 {
		t.Errorf("want an empty state, got %q, err %v", got, err)
	}
}

// A state file that cannot be read must be an error, not an empty state that
// would then be restored over the real one.
func TestCompressedTFStateReportsAFileItCannotRead(t *testing.T) {
	dir := t.TempDir()
	for name, path := range map[string]string{
		"a file that is not there":   filepath.Join(dir, "missing.json"),
		"a path that is a directory": dir,
	} {
		t.Run(name, func(t *testing.T) {
			compressed, err := compressedTFState(path)
			if err == nil {
				t.Fatalf("want an error, got %d bytes and no error", len(compressed))
			}
		})
	}
}
