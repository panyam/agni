# Regenerates boards/jetson-cut from the fetched samples corpus (make samples-oracle at the repo
# root), with kicad-cli on the PATH. The two boxes are in each sheet's own millimetres, and the parts
# they hold are listed in README.md.
#
# It cuts twice. The first cut ends some nets, and the cut schematic names those for itself. So
# kicad-cli netlists the first cut, in a scratch copy under the ignored bin/ so its .kicad_prl stays
# out of the tree, and the second cut renames the board's nets to match, as Update PCB from
# Schematic would.
ROOT := ../..
OUT := examples/cross-probe-project/boards/jetson-cut
SCRATCH := bin/kicadcut-scratch
CUT := go run ./tools/kicadcut -src tools/samples/boards/jetson-agx-thor-baseboard \
	-root jetson-agx-thor-baseboard.kicad_sch -stem jetson-cut -out $(OUT) \
	-sheet usb_debug_pd=105,55,170,115 -sheet power=240,40,300,110

.PHONY: cut odb
cut:
	cd $(ROOT) && $(CUT)
	cd $(ROOT) && mkdir -p $(SCRATCH) && cp $(OUT)/*.kicad_* $(SCRATCH)/ && \
		kicad-cli sch export netlist --format kicadxml -o $(SCRATCH)/net.xml $(SCRATCH)/jetson-cut.kicad_sch
	cd $(ROOT) && $(CUT) -netlist $(SCRATCH)/net.xml
	cd $(ROOT) && rm -f $(SCRATCH)/net.xml $(SCRATCH)/jetson-cut.* $(SCRATCH)/power.kicad_sch $(SCRATCH)/usb_debug_pd.kicad_sch && rmdir $(SCRATCH)

# Regenerates boards/jetson-cut-odb from the cut's board, with kicad-cli and 7-Zip (7zz) on the
# PATH. kicad-cli writes the job into a scratch copy under the ignored bin/, and 7-Zip packs it one
# folder down, as a producer sends a job (agni issue 1591).
ODB_OUT := examples/cross-probe-project/boards/jetson-cut-odb/jetson-cut.odb.7z
odb:
	cd $(ROOT) && rm -rf $(SCRATCH) && mkdir -p $(SCRATCH) && cp $(OUT)/jetson-cut.kicad_pcb $(OUT)/jetson-cut.kicad_pro $(SCRATCH)/ && \
		kicad-cli pcb export odb --compression none --precision 6 -o $(SCRATCH)/jetson-cut $(SCRATCH)/jetson-cut.kicad_pcb
	cd $(ROOT) && rm -f $(ODB_OUT) && cd $(SCRATCH) && 7zz a -t7z -ms=on -mx=9 $(CURDIR)/$(ROOT)/$(ODB_OUT) jetson-cut >/dev/null
	cd $(ROOT) && rm -rf $(SCRATCH)
