"""Build rev B of the Jetson baseboard from rev A, as a documented edit.

The upstream design is one snapshot, so the tutorial's second revision is made here, by a change a
reviewer would plausibly ask for after the first review:

  1. R357 and R358, 2.2 kOhm pull-ups to +1V8 on I2C0_SCL_1V8 and I2C3_SDA_1V8, are fitted rather
     than marked do-not-populate.
  2. The two active-low interrupt nets spelled with a trailing n, PDC_I2C1_IRQn and PDC_I2C2_IRQn,
     take KiCad's overbar, as the project's naming rule asks. Each is the PD controller's side of a
     series resistor whose far side is already ~{PDC_I2Cn_IRQ}, so they take the _R suffix the board
     uses for that shape elsewhere (~{USBC2_INT_R}) rather than a name that would join the two nets.

Usage: python3 tools/make_rev_b.py designs/jetson designs/jetson-rev-b

It copies every file of rev A except design.yaml into the rev B folder, then edits the copies. Each
edit must apply exactly once, so an upstream change to the board fails here rather than producing a
rev B that silently differs from what the tutorial describes.
"""

import pathlib
import re
import shutil
import sys


def edit_once(text, old, new, what):
    if text.count(old) != 1:
        sys.exit(f"make_rev_b: expected exactly one {what}, found {text.count(old)}")
    return text.replace(old, new)


def fit(text, ref):
    # A symbol block opens with "\t(symbol\n" at column one and states its dnp flag before its
    # Reference property, so the flag nearest above the reference is this part's.
    at = text.index(f'(property "Reference" "{ref}"')
    start = text.rindex("\n\t(symbol\n", 0, at)
    block = text[start:at]
    if block.count("(dnp yes)") != 1:
        sys.exit(f"make_rev_b: {ref} is not marked do-not-populate as expected")
    block = block.replace("(dnp yes)", "(dnp no)").replace("(in_bom no)", "(in_bom yes)")
    return text[:start] + block + text[at:]


def main(src, dst):
    src, dst = pathlib.Path(src), pathlib.Path(dst)
    dst.mkdir(parents=True, exist_ok=True)
    for f in src.iterdir():
        if f.name != "design.yaml":
            shutil.copy2(f, dst / f.name)

    som_io = dst / "som_io.kicad_sch"
    text = som_io.read_text()
    for ref in ("R357", "R358"):
        text = fit(text, ref)
    som_io.write_text(text)

    pd = dst / "usb_debug_pd.kicad_sch"
    text = pd.read_text()
    for n in (1, 2):
        new = f"~{{PDC_I2C{n}_IRQ_R}}"
        if new in text:
            sys.exit(f"make_rev_b: {new} already names a net, so the rename would join two nets")
        text = edit_once(text, f'(label "PDC_I2C{n}_IRQn"', f'(label "{new}"', f"PDC_I2C{n}_IRQn label")
    pd.write_text(text)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
