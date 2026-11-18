package projects

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/panyam/agni/service"
)

// PackFromFiles keeps the files a project.yaml's tiers name and leaves out the rest of its folder,
// and refuses a folder that makes no pack (agni issue 1697).
func TestPackFromFilesKeepsWhatTheTiersName(t *testing.T) {
	folder := func(descriptor string) ([]string, func(string) ([]byte, error)) {
		files := map[string]string{
			"project.yaml":          descriptor,
			"profiles/can.yaml":     "name: can\n",
			"params/a.textproto":    "mpn: \"A\"\n",
			"designs/g/g.edn":       "(edif g)",
			"designs/g/design.yaml": "name: g\n",
		}
		var paths []string
		for p := range files {
			paths = append(paths, p)
		}
		slices.Sort(paths)
		return paths, func(p string) ([]byte, error) {
			b, ok := files[p]
			if !ok {
				return nil, os.ErrNotExist
			}
			return []byte(b), nil
		}
	}
	paths, read := folder("name: house\ntitle: House rules\nprofiles: [profiles]\nparams: [params/a.textproto]\n")
	id, pack, kept, err := PackFromFiles(paths, read)
	if err != nil {
		t.Fatal(err)
	}
	if id != "house" || pack.GetName() != "packs/house" || pack.GetTitle() != "House rules" {
		t.Errorf("pack = %q %s %q; want house, packs/house, House rules", id, pack.GetName(), pack.GetTitle())
	}
	if !slices.Equal(kept, []string{"params/a.textproto", "profiles/can.yaml"}) {
		t.Errorf("kept = %v; want the profile and the param, and no design file", kept)
	}
	if !slices.Equal(pack.GetConfig().GetProfileUris(), []string{"profiles"}) || !slices.Equal(pack.GetConfig().GetParamUris(), []string{"params/a.textproto"}) {
		t.Errorf("config tiers = %v %v; want the paths as written", pack.GetConfig().GetProfileUris(), pack.GetConfig().GetParamUris())
	}
	for name, descriptor := range map[string]string{
		"an extends":            "name: house\nextends: ../base/project.yaml\n",
		"a tier outside":        "name: house\nprofiles: [../elsewhere]\n",
		"a tier naming nothing": "name: house\nprofiles: [missing]\n",
	} {
		paths, read := folder(descriptor)
		if _, _, _, err := PackFromFiles(paths, read); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("a project.yaml with %s = %v; want ErrInvalidArgument", name, err)
		}
	}
	if _, _, _, err := PackFromFiles([]string{"profiles/can.yaml"}, read); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a folder with no project.yaml = %v; want ErrInvalidArgument", err)
	}
}
