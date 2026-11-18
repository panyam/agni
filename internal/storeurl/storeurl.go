package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// The store and the uploads mount are named by URL (agni issues 1732, 1739). The scheme picks the
// adapter, so a later sqlite:// or s3:// registers here without a new flag.

// storeSchemes are the --store schemes this build knows, each with what it does or why it is refused.
var storeSchemes = map[string]string{
	"file":   "",
	"sqlite": "is reserved for agni issue 1733 and not built yet",
}

// uploadsSchemes are the --uploads schemes this build knows.
var uploadsSchemes = map[string]string{
	"file": "",
	"s3":   "is reserved for agni issue 1730 and not built yet",
	"gs":   "is reserved for agni issue 1730 and not built yet",
}

// folderOf reads a --store or --uploads value and returns the folder a file:// URL names. An empty
// value returns "". A bare path is refused with the file:// form it meant, a scheme this build knows
// but has not built is refused with its ticket, and any other scheme is refused with the list.
func folderOf(flag, value string, schemes map[string]string) (string, error) {
	if value == "" {
		return "", nil
	}
	scheme, rest, ok := strings.Cut(value, "://")
	if !ok {
		abs, err := filepath.Abs(value)
		if err != nil {
			abs = value
		}
		return "", fmt.Errorf("--%s %q names a path with no scheme; write it as %s", flag, value, fileURL(abs))
	}
	why, known := schemes[scheme]
	if !known {
		names := make([]string, 0, len(schemes))
		for s := range schemes {
			names = append(names, s+"://")
		}
		sort.Strings(names)
		return "", fmt.Errorf("--%s %q: this build knows %s", flag, value, strings.Join(names, ", "))
	}
	if why != "" {
		return "", fmt.Errorf("--%s %q: %s:// %s", flag, value, scheme, why)
	}
	u, err := url.Parse(scheme + "://" + rest)
	if err != nil {
		return "", fmt.Errorf("--%s %q: %w", flag, value, err)
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("--%s %q names the host %q; a file:// URL names a folder on this machine, as file:///srv/agni", flag, value, u.Host)
	}
	if u.Path == "" || !strings.HasPrefix(u.Path, "/") {
		return "", fmt.Errorf("--%s %q names no absolute folder; write it as file:///srv/agni", flag, value)
	}
	p := u.Path
	// A Windows file URL names its drive after the slash, file:///C:/agni, which is the folder C:\agni.
	if len(p) >= 3 && p[2] == ':' && filepath.VolumeName(p[1:]) != "" {
		p = p[1:]
	}
	return filepath.FromSlash(p), nil
}

// fileURL is the file:// URL of an absolute folder: file:///srv/agni, or file:///C:/agni on Windows.
func fileURL(abs string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(abs), "/")
}

// storeFolders reads --store and --uploads together. Uploads need a store to record what they hold,
// so --uploads alone is refused. A store alone keeps records, such as review runs and snapshots, and
// refuses a design's files.
func storeFolders(store, uploads string) (storeDir, uploadsDir string, err error) {
	if storeDir, err = folderOf("store", store, storeSchemes); err != nil {
		return "", "", err
	}
	if uploadsDir, err = folderOf("uploads", uploads, uploadsSchemes); err != nil {
		return "", "", err
	}
	if uploadsDir != "" && storeDir == "" {
		return "", "", fmt.Errorf("--uploads %q needs --store, which records what the uploads hold", uploads)
	}
	return storeDir, uploadsDir, nil
}
