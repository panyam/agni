// Command follow-a-bucket is the walkthrough of mounting an S3 prefix on a server (agni issue
// 1730): an AWS connection, a snapshot of s3://designs/lab served from a folder, and the version
// pinning that keeps a mount's files as they were read. It closes with how a SharePoint folder
// mounts and subscribes to Graph's change notifications (agni issue 1729). The narration is in
// the sidecar walkthrough.md, so this file only binds the steps that run engine code.
//
// The Makefile has four run modes. `make run` prints plain text, `make demo` draws TUI boxes,
// `make runquiet` takes non-interactive defaults (CI-safe), and `make doc` renders to markdown.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/service"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// netlist is the bundled board the bucket holds.
const netlist = "../common/designs/demo-project/designs/mixer/mixer.edn"

func main() {
	root, err := os.MkdirTemp("", "agni-follow-a-bucket-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(root)
	t, err := newTour(root, netlist)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer t.close()
	ctx := context.Background()
	demo := demokit.New("follow-a-bucket").Dir("follow-a-bucket").FromMarkdownBytes(walkthroughMD)

	var lab service.MountInfo
	demo.Bind("bucket").Run(func(demokit.StepContext) *demokit.StepResult {
		m, files, err := t.bucket(ctx)
		if err != nil {
			return demokit.Errf("mount the bucket: %v", err)
		}
		lab = m
		fmt.Printf("mount %s, kind %s, from %s\nfiles in its root: %v\n", m.Name, m.Kind, m.Source, files)
		return nil
	})

	demo.Bind("pinned").Run(func(demokit.StepContext) *demokit.StepResult {
		same, err := t.pinned(lab)
		if err != nil {
			return demokit.Errf("read through the mount: %v", err)
		}
		fmt.Printf("the bucket's mixer.edn was overwritten; the mount still serves the netlist it read: %v\n", same)
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
