package rendition

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// LibreOffice's -env:UserInstallation takes a file URL. Prefixing "file:///" to
// an absolute Unix path gave file:////tmp/x.
func TestFileURI_HasExactlyThreeSlashesBeforeThePath(t *testing.T) {
	dir := t.TempDir()

	got := fileURI(dir)

	if !strings.HasPrefix(got, "file:///") || strings.HasPrefix(got, "file:////") {
		t.Fatalf("fileURI(%q) = %q", dir, got)
	}
	if runtime.GOOS != "windows" && got != "file://"+filepath.ToSlash(dir) {
		t.Fatalf("fileURI(%q) = %q", dir, got)
	}
}
