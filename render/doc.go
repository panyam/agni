// Package render turns the geometry sidecar (agni.v1.geom) into concrete drawable
// output. It is core, runtime-agnostic Go (CONSTRAINTS C1): it takes the geom proto and
// returns bytes/strings, with no syscall/js, no file paths, and no DB handles. The same
// code backs the SVG verification dumper here, the netlist-vs-geometry oracle, and the
// eventual WebGL vertex packer. The placement transform math it draws with lives in
// internal/geomath, shared with the readers that compute pin world positions (C15).
package render
