package kicad

import "bytes"

// SheetFiles lists every sub-sheet file a schematic's hierarchy references, each as its Sheetfile
// path relative to the root's folder, in the order the walk first meets it. open fetches a sub-sheet
// by that path, as the hierarchy readers' open does, and a sheet that will not open or parse is
// listed but not walked into. The root itself is not listed.
//
// It answers whether a file someone named is one sheet of a larger design, so a read of that sheet
// alone can say so (agni issue 1023).
func SheetFiles(rootContent []byte, open func(relPath string) ([]byte, error)) []string {
	var out []string
	seen := map[string]bool{}
	var visit func(content []byte, depth int)
	visit = func(content []byte, depth int) {
		if depth > 64 {
			return
		}
		root, err := parse(bytes.NewReader(content))
		if err != nil {
			return
		}
		for _, sub := range root.Children("sheet") {
			file := propValue(sub, "Sheetfile")
			if file == "" || seen[file] {
				continue
			}
			seen[file] = true
			out = append(out, file)
			if child, err := open(file); err == nil {
				visit(child, depth+1)
			}
		}
	}
	visit(rootContent, 0)
	return out
}
