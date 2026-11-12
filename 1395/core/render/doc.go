// Package render turns the geometry sidecar (agni.v1.geom) into drawable output. It is
// core, runtime-agnostic Go (CONSTRAINTS C1), taking the geom proto and returning bytes or
// strings with no syscall/js, no file paths, and no DB handles. The same code backs the SVG
// dumper, the netlist-vs-geometry oracle, and the WebGL vertex packer (pack.go). Placement
// transform math lives in internal/geomath, shared with the readers that compute pin world
// positions (C17).
package render
