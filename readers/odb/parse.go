package odb

import (
	"fmt"
	"io/fs"
	"strings"
)

// parsedJob is everything both reads share: the job, its eda/data, and the components of each side
// in file order, which is the numbering eda/data's TOP subnets use.
type parsedJob struct {
	*job
	eda     *eda
	sides   [][]component // one slice per component layer, in componentLayers order
	sideTop []bool        // whether each side is the front
	info    map[string]string
}

// parseJob opens a job and reads the files both the netlist and the board need.
func parseJob(fsys fs.FS) (*parsedJob, error) {
	j, err := openJob(fsys)
	if err != nil {
		return nil, err
	}
	p := &parsedJob{job: j, info: map[string]string{}}
	if b, err := j.readOptional("misc/info"); err != nil {
		return nil, err
	} else if b != nil {
		if blocks, perr := parseStructured(b); perr == nil && len(blocks) > 0 {
			p.info = blocks[0].kv
		}
	}
	b, err := j.readOptional(j.stepFile("eda/data"))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("odb: step %q has no eda/data, so the job carries no netlist", j.step)
	}
	if p.eda, err = parseEDA(b); err != nil {
		return nil, err
	}
	names, back := j.matrix.componentLayers()
	for i, name := range names {
		cb, err := j.readOptional(j.layerFile(name, "components"))
		if err != nil {
			return nil, err
		}
		comps, err := parseComponents(cb, back[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", strings.ToLower(name), err)
		}
		p.sides = append(p.sides, comps)
		p.sideTop = append(p.sideTop, !back[i])
	}
	return p, nil
}

// allComponents lists every component, front side first.
func (p *parsedJob) allComponents() []*component {
	var out []*component
	for i := range p.sides {
		for k := range p.sides[i] {
			out = append(out, &p.sides[i][k])
		}
	}
	return out
}

// jobName is the job's name as misc/info states it.
func (p *parsedJob) jobName() string {
	return p.info["JOB_NAME"]
}
