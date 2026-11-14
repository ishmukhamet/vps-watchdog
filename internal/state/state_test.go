package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomicReplacesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	for _, want := range []string{"first", "second"} {
		if err := WriteAtomic(path, []byte(want)); err != nil {
			t.Fatalf("WriteAtomic: %v", err)
		}
		got, err := ReadSmall(path, 64)
		if err != nil || string(got) != want {
			t.Fatalf("ReadSmall = %q, %v", got, err)
		}
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file left behind: %v", err)
	}
}

func TestReadSmallRejectsLargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 65)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSmall(path, 64); err == nil {
		t.Fatal("expected size error")
	}
}

func TestRemoveIgnoresMissingFile(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}
