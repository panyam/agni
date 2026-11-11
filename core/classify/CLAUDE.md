# core/classify

This file covers part-number stamping, component classes, the device-class evidence tiers, and values read from part names.

- **A reader records a part number where its grammar puts it and never promotes it.** `classify.StampMPN` is the shared ingestion pass that fills `ir.Component.mpn` (agni 519). It reads the component's own aliases first and `ir.PartType.mpn` second.
- **A new MPN spelling goes in `classify.MPNAliases`, never in a reader.** A project's own spelling goes in its lexicon's `attribute.mpn` (agni 1285).
- **`StampMPN` runs in `DeriveDesign`**, because it reads configuration. KiCad keeps a symbol's other fields on the section, so classification never reads them.
- **A value written as a library's part name is read from the name at ingestion** (`ValueVocab.valueFromName`, agni 1115). `R_10k_0402` and `C_4u7_25V_0402` are examples. The value is one token with an SI prefix or a unit, in the class's own unit. A package size (`0402`) or a rating (`25V`) never reads as the value, and a name with two different values reads as none.
- **A thermistor is a `thermistor` and a `resistor`** (agni 627). `ferrite` and `inductor` use the same family shape. A part classed UNKNOWN emits no `component.class` row, so it falls out of every class-scoped rule and query.
- **A project can extend a shipped class and cannot declare a new one.** `lexicon.class` takes `patterns` and ref-des `prefixes` keyed by a shipped class. A class name the engine does not know is a load error.
- **The class names a config may use derive from `model.ComponentClasses`** (agni 677). A test reads the const block and holds the list to it. DECISIONS.md says a new class would come as a registry, once a project asks for one.
- **A device class has two evidence tiers.** `ir.Component.device_classes` is a set of tags, and each tag carries a `ClassSource` (agni 710). This is the second use of C9's evidence-tier variant, after `ir.Net.roles`.
- **`classify.Stamp` is the convention tier and `classify.StampClassesFromSpecs` is the datasheet tier.** Only the datasheet tier separates a crystal from a ceramic resonator or names an `ideal_diode_controller`. The structural path cannot resolve either one.
- **`Loader.ReadDesign` needs two orderings, and getting either wrong is silent.** The convention pass replaces the set, so it erases any tag written before it. The datasheet pass joins on the MPN that `StampMPN` fills, so it runs after `StampMPN`.
- **The datasheet tier runs in two places on purpose.** The Loader runs it when the read carries a corpus. `check.Model` runs it when it receives a corpus that the read did not have. Two producers are safe because the pass is additive and idempotent. A third producer is a bug.
- **`classify.BySpecificity` picks the headline class among several true ones, and `MostSpecific` is its head.** The drawing and the model both rank through it, so they agree. An unranked vendor class such as `regulator` sorts behind the keyword class and does not displace it.
- **A second tier must never be the only tier in the set**, because an empty set tells a consumer to re-derive. So `WithParamProvider` runs the convention pass first on a design that reaches it unstamped.
