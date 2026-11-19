package storeurl

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStoreAndUploadsTakeFileURLs(t *testing.T) {
	for _, tc := range []struct {
		store, uploads string
		wantStore      string
		wantUploads    string
		wantErr        string
	}{
		{store: "", uploads: ""},
		{store: "file:///srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "file:///srv/agni/meta", uploads: "file:///srv/agni/files", wantStore: "/srv/agni/meta", wantUploads: "/srv/agni/files"},
		{store: "file://localhost/srv/agni/meta", wantStore: "/srv/agni/meta"},
		{store: "/srv/agni/meta", wantErr: "names a path with no scheme; write it as file:///"},
		{store: "sqlite:///srv/agni.db", wantErr: "agni issue 1733"},
		{store: "postgres://db/agni", wantErr: "this build knows file://, sqlite://"},
		{store: "file://server/srv/agni", wantErr: "names the host"},
		{store: "file://relative", wantErr: "names the host"},
		{store: "file:///srv/agni/meta", uploads: "s3://bucket/agni", wantErr: "agni issue 1730"},
		{uploads: "file:///srv/agni/files", wantErr: "needs --store"},
	} {
		store, uploads, err := Folders(tc.store, tc.uploads)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("--store %q --uploads %q = %v, want an error naming %q", tc.store, tc.uploads, err, tc.wantErr)
			}
			continue
		}
		if err != nil || store != filepath.FromSlash(tc.wantStore) || uploads != filepath.FromSlash(tc.wantUploads) {
			t.Errorf("--store %q --uploads %q = %q, %q, %v; want %q, %q", tc.store, tc.uploads, store, uploads, err, tc.wantStore, tc.wantUploads)
		}
	}
	if runtime.GOOS == "windows" {
		if got, err := folderOf("store", "file:///C:/agni/meta", storeSchemes); err != nil || got != `C:\agni\meta` {
			t.Errorf("--store file:///C:/agni/meta = %q, %v; want C:\\agni\\meta", got, err)
		}
	}
}
