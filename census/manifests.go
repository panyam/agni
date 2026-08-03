package census

// This file is the reviewed classification of every source construct the readers encounter,
// seeded from the reader-coverage audit (private research repo docs/18) and completed over the
// private corpus with `agni census`. Fixture-present constructs MUST be classified or the CI
// census fails; corpus-only constructs are classified so `agni census` reaches a clean baseline
// and the known drops are tracked. When a reader starts consuming a construct, flip its entry to
// Consumed here and the diff shows the coverage change.

// classification helpers keep the manifests terse.
func co(why string) Entry         { return Entry{Consumed, why, ""} }
func dd(why string) Entry         { return Entry{DroppedByDesign, why, ""} }
func dc(why, ticket string) Entry { return Entry{DroppedCosmetic, why, ticket} }
func da(why, ticket string) Entry { return Entry{DroppedAnalysis, why, ticket} }
func dl(why, ticket string) Entry { return Entry{DroppedLatent, why, ticket} }

// bulk-fill a shared classification across many tokens (the editor/config tails).
func fill(m map[string]Entry, e Entry, toks ...string) {
	for _, t := range toks {
		m[t] = e
	}
}

// Manifests returns every format census, keyed by format name. This is the registry the CI test
// and the `agni census` CLI both drive.
func Manifests() []Manifest {
	return []Manifest{kicadPCB(), kicadSch(), edifManifest(), ipc2581(), xschemManifest(), gedaManifest()}
}

func kicadPCB() Manifest {
	m := map[string]Entry{
		"kicad_pcb": co("board root"), NumberToken: co("layer-table rows (numeric-keyed) + version triples"),
		"layer": co("feature layer"), "layers": co("layer table / pad layer set"), "net": co("net table + copper net refs"),
		"net_name": co("zone net name"), "footprint": co("component placement"), "at": co("placement/pad/text position"),
		"uuid": co("provenance SourceId"), "property": co("Reference/Value -> attributes + BoardText"),
		"pad": co("copper land"), "size": co("pad extents / font size"), "drill": co("pad/via hole"),
		"segment": co("copper track"), "via": co("inter-layer via"), "start": co("segment/graphic endpoint"),
		"end": co("segment/graphic endpoint"), "mid": co("arc midpoint"), "width": co("track/stroke width"),
		"xy": co("polygon point"), "pts": co("polygon/zone points"), "polygon": co("zone/outline polygon"),
		"zone": co("copper pour (authored outline)"), "title_block": co("design name"), "title": co("design name"),
		"gr_line": co("board graphic / Edge.Cuts outline"), "gr_arc": co("board graphic / outline arc"),
		"gr_rect": co("board graphic / outline rect"), "gr_text": co("free text (title block)"),
		"gr_poly": co("board graphic polygon"),
		"fp_line": co("footprint silk/fab graphic"), "fp_circle": co("footprint silk/fab graphic"),
		"fp_arc": co("footprint silk/fab graphic"), "fp_poly": co("footprint silk/fab graphic"),
		"fp_rect": co("footprint silk/fab graphic"),
		"font":    co("text style"), "effects": co("text style"), "justify": co("text justify"),
		"center": co("circle center"), "fill": co("graphic fill"), "stroke": co("graphic stroke width"),
		"version": dd("file-format version"), "generator": dd("authoring-tool tag"),
		"type": dc("stroke type (solid/dash) not carried", ""),
		// gaps with tickets:
		"arc":                    dl("curved copper tracks dropped (NetCopper is straight-only)", "WS1-035"),
		"filled_polygon":         da("zone FILL geometry dropped; only the authored outline is kept", "WS1-031"),
		"filled_areas_thickness": da("zone FILL geometry dropped; only the authored outline is kept", "WS1-031"),
		"fp_text":                da("footprint user text (${REFERENCE}/${VALUE} fab marks) not read", "WS1-037"),
		"attr":                   da("footprint mount-type / DNP / exclude flags dropped", "WS1-037"),
		"clearance":              da("board clearance rule not read (re-derived, not declared)", ""),
		"pad_to_mask_clearance":  da("board-wide mask clearance default not read", ""),
		"teardrops":              dc("teardrop pad/via fillets not read", ""),
		"gr_text_box":            dc("free text box not read (only gr_text)", ""),
		"group":                  dc("design grouping lost", ""), "members": dc("group membership lost", ""),
		"model":   dd("3D model reference (out of a 2D layout/DRC engine)"),
		"setup":   dd("board setup section (thresholds live in child heads, classified individually)"),
		"general": dd("board general section (thickness/metadata)"),
	}
	// stackup material / dielectric (the IPC-carries-what-KiCad-drops gap):
	fill(m, da("stackup material / dielectric / thickness dropped", "WS1-036"),
		"stackup", "epsilon_r", "material", "loss_tangent", "dielectric_constraints", "min_thickness", "thickness", "copper_finish")
	// zone fill / thermal config (part of the zone-fill gap):
	fill(m, dc("zone fill/thermal config not read (only the authored outline is)", "WS1-031"),
		"filling", "hatch", "island_area_min", "island_removal_mode", "thermal_bridge_angle", "thermal_bridge_width",
		"thermal_gap", "zone_connect", "connect_pads", "filter_ratio", "priority", "min_length", "max_length", "max_width",
		"best_length_ratio", "best_width_ratio", "prefer_zone_connections")
	// pad/teardrop shape refinements:
	fill(m, dc("pad/teardrop shape refinement not carried", ""),
		"roundrect_rratio", "teardrop", "legacy_teardrops", "chamfer", "allow_two_segments", "curved_edges")
	// editor / plot / footprint-editor metadata (no reader-relevant vocabulary):
	fill(m, dd("editor/plot/footprint configuration or metadata"),
		"pcbplotparams", "aux_axis_origin", "grid_origin", "allow_soldermask_bridges_in_footprints",
		"duplicate_pad_numbers_are_jumpers", "embedded_fonts", "generator_version", "remove_unused_layers", "tenting",
		"capping", "covering", "plugging", "anchor", "options", "primitives", "pins", "point", "scale", "rotate",
		"pinfunction", "pintype", "sheetfile", "sheetname", "path", "unit", "units", "xyz", "locked", "unlocked",
		"hide", "enabled", "name", "offset", "color", "comment", "company", "date", "rev", "descr", "tags",
		"paper", "border", "back", "front", "free")
	return Manifest{Format: "kicad-pcb", Kind: KindKiCad, Entries: m}
}

func kicadSch() Manifest {
	m := map[string]Entry{
		"kicad_sch": co("schematic root"), "at": co("placement/pin/label position"), "center": co("circle center"),
		"circle": co("symbol graphic"), "arc": co("symbol arc graphic"), "mid": co("arc midpoint"),
		"effects": co("text style"), "end": co("wire/graphic endpoint"),
		"fill": co("shape fill"), "font": co("text style"), "hide": co("field visibility"),
		"hierarchical_label": co("hierarchical port -> net"), "instances": co("per-instance ref-des"),
		"junction": co("net junction"), "justify": co("text justify"), "label": co("local/global label -> net"),
		"length": co("pin length"), "lib_id": co("part reference"), "lib_symbols": co("part types + pins"),
		"mirror": co("placement transform"), "name": co("net/pin/symbol name"), "no_connect": co("no-connect marker"),
		"number": co("pin number"), "offset": co("pin-name offset"), "paper": co("page size"),
		"path": co("instance path"), "pin": co("symbol pin"), "pin_names": co("pin-name style"),
		"pin_numbers": co("pin-number style"), "polyline": co("symbol/wire graphic"), "power": co("power symbol"),
		"project": co("instance project scope"), "property": co("Reference/Value/MPN -> attributes + fields"),
		"pts": co("wire/shape points"), "radius": co("arc radius"), "rectangle": co("symbol graphic"),
		"reference": co("ref-des"), "sheet": co("hierarchy sub-sheet"), "size": co("font size"),
		"start": co("wire/graphic endpoint"), "stroke": co("graphic stroke width"), "symbol": co("component/lib symbol"),
		"text": co("free text -> label"), "title": co("design name"), "title_block": co("design name"),
		"unit": co("multi-unit symbol section"), "uuid": co("provenance"), "wire": co("net wire"),
		"xy": co("point"), "value": co("symbol value graphic"), "page": co("sheet page number"),
		"version": dd("file-format version"), "generator": dd("authoring-tool tag"),
		// cosmetic:
		"comment":  dc("title-block comment field not carried", ""),
		"diameter": dc("junction dot diameter drawn at default", ""),
		"type":     dc("stroke type not carried", ""),
		"width":    dc("wire/shape stroke width not carried (drawn at renderer default)", ""),
		// consumed (WS1-037):
		"footprint": co("Footprint/Datasheet property -> component attributes"),
		"shape":     da("hierarchical-port direction (input/output/bidir) dropped", "WS1-037"),
		// bus constructs: detected + flagged as unmodeled (WS1-034 Phase 1); member expansion is Phase 2.
		"bus":       co("bus wire detected + flagged unmodeled (WS1-034 P1); member expansion P2"),
		"bus_entry": co("bus-entry tap detected + flagged unmodeled (WS1-034 P1); member expansion P2"),
		"bus_alias": co("bus-alias detected + flagged unmodeled (WS1-034 P1); member expansion P2"),
		"members":   dl("bus-alias member list unread until expansion", "WS1-034"),
		"dnp":    co("DNP fabrication flag -> component attributes"),
		"in_bom": co("BOM-scope flag -> component attributes"), "on_board": co("assembly-scope flag -> component attributes"),
		"netclass": co("net_class populated from .kicad_pro net_settings; local netclass_flag directive still unread"),
		"image":    co("embedded raster image"), "global_label": co("global label -> net"),
		"text_box":       dc("note-box frame dropped (text drawn)", ""),
		"embedded_fonts": dd("editor font-embedding metadata"), "exclude_from_sim": dd("sim-scope flag, no consumer"),
	}
	fill(m, dc("symbol de Morgan alternate body not selected", ""), "body_style", "body_styles")
	fill(m, dc("text style (weight/color/margins) not carried", ""), "bold", "italic", "color", "thickness", "margins", "show_name")
	fill(m, dd("editor / legacy-instance / metadata (no reader consumer)"),
		"default_instance", "do_not_autoplace", "duplicate_pin_numbers_are_jumpers", "fields_autoplaced",
		"generator_version", "id", "in_pos_files", "sheet_instances", "symbol_instances", "lib_name", "net_chain",
		"company", "date", "rev")
	return Manifest{Format: "kicad-sch", Kind: KindKiCad, Entries: m}
}

func edifManifest() Manifest {
	m := map[string]Entry{
		"edif": co("root"), "edifVersion": co("format version -> attr"), "edifLevel": co("format level"),
		"design": co("design + top cell"), "library": co("part library"), "cell": co("cell -> part type"),
		"cellType": co("cell type"), "view": co("view"), "viewType": co("view type"), "viewRef": co("section view ref"),
		"cellRef": co("part ref"), "libraryRef": co("library ref"), "interface": co("cell interface (ports)"),
		"contents": co("cell contents (instances/nets)"), "instance": co("component section"),
		"instanceRef": co("net connection ref"), "port": co("port -> pin"), "portInstance": co("port->pin mapping"),
		"portRef": co("net connection"), "net": co("net"), "joined": co("net connectivity"),
		"property": co("instance parameter -> attributes"), "designator": co("ref-des"),
		"direction": co("pin direction"), "name": co("entity name"), "rename": co("name display form"),
		"string": co("string value / display"), "member": co("bus-element pin identity"),
		"page": co("schematic page"), "pageSize": co("page size"), "symbol": co("symbol graphics"),
		"boundingBox": co("symbol bbox"), "figure": co("symbol figure"), "rectangle": co("figure rect"),
		"arc": co("figure arc"), "openShape": co("figure open path"), "curve": co("figure curve"),
		"path": co("figure path"), "pointList": co("figure points"), "dot": co("figure dot"),
		"connectLocation": co("pin connect point"), "portImplementation": co("placed port/pin"),
		"origin": co("placement origin"), "orientation": co("placement orientation"),
		"transform": co("placement transform"), "scale": co("unit scale"), "scaleX": co("scale x"),
		"scaleY": co("scale y"), "textHeight": co("text height"), "justify": co("text justify"),
		"display": co("display attributes"), "stringDisplay": co("placed string"),
		"annotate": co("annotation figure"), "commentGraphics": co("comment figures"),
		"numberDefinition": co("distance-unit scaling"), "technology": co("technology (unit scaling)"),
		"unit": co("unit definition"), "pt": co("point"), "figureGroupOverride": co("per-figure style override"),
		"visible": co("field visibility"), "circle": co("figure circle"), "figureGroup": co("figure-group style"),
		// corpus-only gaps:
		"propertyDisplay": co("placed property value (inline stringDisplay) -> SymbolPlacement.fields"),
		"comment":         dd("EDIF free comment / documentation string, not read"),
		"keywordDisplay":  dc("keyword display text not read", ""),
		"color":           dc("authored figure color not carried (monochrome by figureGroup)", "WS1-037"),
		"pathWidth":       dc("per-wire stroke width flattened", ""),
		"borderPattern":   dc("page border style not carried", ""),
		"status":          da("export provenance (timestamp/author/tool) dropped", "WS1-037"),
		"written":         da("export timestamp/tool dropped", "WS1-037"),
		"author":          da("export author dropped", "WS1-037"), "program": da("export tool/version dropped", "WS1-037"),
		"timestamp":        da("export timestamp dropped", "WS1-037"),
		"array":            co("bus (array) port detected + flagged unmodeled (WS1-034 P1); member expansion P2"),
		"offPageConnector": da("off-page connector net-link not modeled", "WS1-034"),
	}
	fill(m, dd("EDIF syntactic value primitive (number/boolean/self-ref/short atom)"),
		"e", "a", "an", "this", "true", "false", "boolean")
	fill(m, dd("EDIF back-annotation / keyword-map / view-map / grid / authorship metadata"),
		"globalPortRef", "gridMap", "instanceBackAnnotate", "portBackAnnotate", "keywordLevel", "keywordMap",
		"owner", "userData", "version", "viewMap")
	return Manifest{Format: "edif", Kind: KindEDIF, Entries: m}
}

func ipc2581() Manifest {
	m := map[string]Entry{
		"IPC-2581": co("root"), "Content": co("content section"), "Ecad": co("ecad data"), "CadData": co("cad data"),
		"Step": co("board step"), "Profile": co("board outline"), "Polygon": co("outline/feature polygon"),
		"Polyline": co("copper track polyline"), "PolyBegin": co("polyline start point"),
		"PolyStepSegment": co("straight polyline step"), "PolyStepCurve": co("arc polyline step (outline)"),
		"LayerFeature": co("per-layer features"), "Set": co("feature set (net copper)"), "Features": co("features container"),
		"Pad": co("copper pad"), "Hole": co("drill / via"), "Circle": co("pad primitive"), "RectCenter": co("pad primitive"),
		"Location": co("feature location"), "Xform": co("placement transform (rotation)"),
		"Component": co("placement"), "Package": co("footprint (pins)"), "Pin": co("package pin"),
		"RefDes": co("ref-des"), "LogicalNet": co("net -> copper join"), "PinRef": co("net pin ref"),
		"Layer": co("layer table"), "Stackup": co("stackup group"), "StackupGroup": co("stackup group"),
		"StackupLayer": co("stackup layer (thickness)"), "Bom": co("BOM"), "BomItem": co("BOM line -> component MPN"),
		"DictionaryStandard": co("standard primitive dictionary"), "EntryStandard": co("primitive definition"),
		"StandardPrimitiveRef": co("primitive reference"), "DictionaryLineDesc": co("line descriptors"),
		"EntryLineDesc": co("line descriptor"), "LineDesc": co("line width"), "LineDescRef": co("line width ref"),
	}
	// WS1-031 producer parity (VALUE + silk/fab graphics + copper fills + via spans + user pads):
	fill(m, co("VALUE -> attrs; silk/fab -> BoardGraphic; Set fill -> Zone; Span -> via layers; user primitive -> pad (WS1-031)"),
		"Marking", "Outline", "AssemblyDrawing", "NonstandardAttribute",
		"Contour", "Span", "UserPrimitiveRef", "DictionaryUser", "EntryUser", "UserSpecial", "Oval")
	fill(m, da("fill cutouts / empty silkscreen-layer features / component metadata not read", "WS1-031"),
		"Cutout", "SilkScreen", "LandPattern", "Property")
	fill(m, dl("padstack def/instance indirection not read (Layer Span used instead); slot/cavity not read", ""),
		"PadStackDef", "PadstackHoleDef", "PadstackPadDef", "SlotCavity")
	fill(m, da("stackup material / dielectric / declared DRC constraints dropped", "WS1-036"),
		"Spec", "SpecRef", "Impedance", "Characteristics", "Conductor", "Dielectric", "General")
	fill(m, dl("curved copper/graphic arc step dropped", "WS1-035"), "Arc")
	fill(m, dc("line/graphic primitive not read", ""), "Line")
	fill(m, dd("physical-net grouping (net binding is via Set@net)"), "PhyNet", "PhyNetPoint", "PhyNetGroup")
	fill(m, dd("logistics / dictionary / metadata (no reader-relevant vocabulary)"),
		"LogisticHeader", "HistoryRecord", "BomHeader", "BomRef", "CadHeader", "Certification", "Color", "ColorRef",
		"Datum", "DictionaryColor", "DictionaryFillDesc", "EntryColor", "EntryFillDesc", "Enterprise", "FileRevision",
		"FillDesc", "FillDescRef", "FunctionMode", "LayerRef", "Person", "PickupPoint", "Role", "SoftwarePackage",
		"StepRef", "Textual")
	return Manifest{Format: "ipc2581", Kind: KindXML, Entries: m}
}

func xschemManifest() Manifest {
	m := map[string]Entry{
		"v": co("version header"), "C": co("component instance -> placement"), "N": co("wire/net segment"),
		"T": co("text -> label"), "L": co("line graphic (symbol)"), "B": co("box/pin graphic"),
		"V": co("polygon/circle graphic (symbol)"),
		"G": dd("global/grid attribute block"), "K": dd("symbol attribute block (type)"),
		"S": dd("SPICE directive block"), "E": dd("verilog/VHDL directive block"), "F": dd("wrapped-content artifact"),
		"@name": co("ref-des"), "@lab": co("net label"), "@value": co("component value"),
		"@device": co("device class"), "@dir": co("pin direction"), "@pinnumber": co("pin number"),
		"@type": co("symbol/pin type"), "@template": co("symbol default params"),
		"@version": dd("format version"), "@file_version": dd("file version"), "@author": dd("author metadata"),
		"@font": dc("text font not carried", ""),
		// gaps:
		"A":  dc("sheet-level arc graphic dropped (SheetGeometry.shapes exists)", "WS1-033"),
		"P":  dc("sheet-level polygon graphic dropped", "WS1-033"),
		"@l": da("device length (L) dropped", ""), "@w": da("device width (W) dropped", ""),
		"@m": da("device multiplicity dropped", ""), "@model": da("SPICE model binding dropped", ""),
		"@node": da("net-node annotation dropped", ""), "@sig_type": da("signal type dropped", ""),
		"@footprint":     da("footprint not in section attrs", "WS1-037"),
		"@spice_ignore":  dl("spice_ignore not honored (device wrongly emitted)", ""),
		"@symbol_ignore": dl("symbol_ignore not honored", ""),
	}
	fill(m, dc("embedded image / display style not rendered", "WS1-033"),
		"@image", "@image_data", "@jpeg_quality", "@layer", "@color", "@alpha", "@text_size_0", "@text_size_1",
		"@linewidth_mult", "@dash", "@xlabmag", "@hide_texts")
	fill(m, dd("simulation / netlist-format / editor directive (no reader consumer)"),
		"@format", "@verilog_format", "@verilog_primitive", "@simulator", "@sim_type", "@analysis", "@tclcommand",
		"@program", "@savecurrent", "@only_toplevel", "@autoload", "@goto", "@ps_invert", "@flags", "@function0",
		"@current", "@voltage", "@area", "@propag", "@url", "@descr", "@comment", "@dataset", "@hilight_wave",
		"@divx", "@divy", "@subdivx", "@subdivy", "@x1", "@y1", "@x2", "@y2", "@unitx", "@xvalue")
	return Manifest{Format: "xschem", Kind: KindLine, Entries: m}
}

func gedaManifest() Manifest {
	return Manifest{Format: "geda", Kind: KindLine, Entries: map[string]Entry{
		"v": co("version header"), "C": co("component -> placement"), "N": co("net segment"),
		"T": co("text / attribute"), "L": co("line graphic (symbol)"), "B": co("box graphic (symbol)"),
		"P": co("pin (symbol)"), "G": co("picture (embedded/external)"),
		"@refdes": co("ref-des"), "@value": co("component value"), "@device": co("device class"),
		"@netname": co("explicit net name"), "@net": co("power/forced net tap (power symbols only)"),
		"@pinnumber": co("pin number"), "@pinlabel": co("pin label"), "@pintype": co("pin type -> direction"),
		"@model-name": co("model name attribute"), "@footprint": co("footprint attribute"),
		"@file": dd("hierarchy source file"),
		// gaps:
		"U":           co("bus object detected + flagged unmodeled (WS1-034 P1); member expansion P2"),
		"V":           dc("circle graphic at sheet level dropped", "WS1-033"),
		"A":           dc("arc graphic at sheet level dropped", "WS1-033"),
		"H":           dc("path graphic not drawn", ""),
		"@slot":       co("multi-gate slot -> section of one folded component + per-slot pin numbers"),
		"@slotdef":    co("slot -> physical pin map (per-slot pin numbering)"),
		"@pinseq":     co("pin sequence -> slot pin-map index"),
		"@numslots":   dd("slot count (derivable from slotdef rows; not needed for resolution)"),
		"@source":     da("hierarchical sub-sheet not descended", "WS1-033"),
		"@symversion": dd("symbol-version metadata"), "@model": da("SPICE model binding dropped", ""),
		"@comment": dc("free comment attribute not carried", ""), "@TEMP": dd("SPICE temperature directive"),
	}}
}
