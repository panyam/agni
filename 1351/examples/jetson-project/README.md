# Jetson tutorial project

The review project the [tutorial ladder](../../docsite/content/tutorials/index.md) teaches on. It
wraps a team's configuration (naming conventions, interface profiles, part parameters, design intent
and a checklist) around Antmicro's open-source baseboard for the NVIDIA Jetson AGX Thor module.

The board is Antmicro's, under Apache-2.0, and is fetched rather than committed:

```
make samples-oracle            # at the repo root, once
make -C examples/jetson-project board
make -C examples/jetson-project stats
```

`board` copies the KiCad files into `designs/jetson/` beside `design.yaml`, the one file in that
folder that is ours, and never overwrites it. The copies are gitignored.

The tutorial's command captures run against this folder with the board copied in the same way (the
`fetched:` field of a capture spec), so a committed edit here regenerates every capture over it.
