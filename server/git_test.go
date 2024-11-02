package server

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/ije/gox/crypto/rs"
)

func TestListRepoRefs(t *testing.T) {
	refs, err := listRepoRefs("https://github.com/esm-dev/esm.sh")
	if err != nil {
		t.Fatal(err)
	}
	var headSha string
	for _, ref := range refs {
		if ref.Ref == "HEAD" {
			headSha = ref.Sha
			break
		}
	}
	if headSha == "" {
		t.Fatal("HEAD not found")
	}
}

func TestGhInstall(t *testing.T) {
	dir := filepath.Join(os.TempDir(), rs.Hex.String(8))
	defer os.RemoveAll(dir)
	err := ghInstall(dir, "esm-dev/esm.sh", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !existsFile(path.Join(dir, "node_modules/esm-dev/esm.sh/README.md")) {
		t.Fatal("README.md not found")
	}
}
