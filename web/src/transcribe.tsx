import { createSignal, For, Show } from "solid-js";
import { LimitKind, PinFunction, type PartSpec, type Parameter, type Pin } from "./gen/agni/v1/param/param_pb.js";
import { REGION_TYPES, type Region, type RegionType } from "./regions.js";
import { paramsForRegion, pinsForRegion, pinProblems, type NewParamFields, type NewPinFields } from "./bank.js";

// The pin functions the editor offers. UNSPECIFIED IS INCLUDED here, unlike the limit kinds above,
// because a pin table may genuinely have no type column and a pin whose name and number are known is
// still worth recording; refusing it would lose the numbering.
const PIN_FUNCTIONS: [PinFunction, string][] = [
  [PinFunction.POWER_INPUT, "Power input"],
  [PinFunction.POWER_OUTPUT, "Power output"],
  [PinFunction.GROUND, "Ground"],
  [PinFunction.INPUT, "Input"],
  [PinFunction.OUTPUT, "Output"],
  [PinFunction.BIDIRECTIONAL, "Bidirectional (I/O)"],
  [PinFunction.PASSIVE, "Passive"],
  [PinFunction.NO_CONNECT, "No connect"],
  [PinFunction.UNSPECIFIED, "Not stated"],
];

// The limit kinds the editor offers, with human labels (UNSPECIFIED is excluded: a manual row must
// classify, since a rule cannot dispatch on an unknown kind).
const LIMIT_LABELS: [LimitKind, string][] = [
  [LimitKind.ABSOLUTE_MAX, "Absolute max"],
  [LimitKind.RECOMMENDED_OPERATING, "Recommended operating"],
  [LimitKind.CHARACTERISTIC, "Characteristic"],
];

// TranscribeHandlers is everything the panel reads and writes, provided by the workbench which owns
// the bank state. The accessors are reactive (they re-read on any bank edit); the mutators persist.
export interface TranscribeHandlers {
  spec: () => PartSpec;
  region: () => Region | null;
  regionType: () => RegionType;
  deletableRegion: () => boolean;
  onDeleteRegion: () => void;
  setType: (t: RegionType) => void;
  setMeta: (patch: Partial<{ mpn: string; manufacturer: string; deviceClass: string }>) => void;
  addParam: (f: NewParamFields) => void;
  deleteParam: (p: Parameter) => void;
  addPin: (f: NewPinFields) => void;
  deletePin: (p: Pin) => void;
  setPinNumber: (pin: Pin, packageRef: string, number: string) => void;
  addPackage: (id: string, name: string, suffix: string) => void;
  deletePackage: (id: string) => void;
  toggleBinding: (p: Parameter, pinId: string) => void;
}

// numOrUndef parses an editor numeric field, leaving it unset (RangeValue has explicit presence) for
// blank or non-numeric input rather than coercing to 0; a missing max is not a max of zero.
function numOrUndef(s: string): number | undefined {
  const t = s.trim();
  if (t === "") return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? n : undefined;
}

// fmtRange renders a parameter's min/typ/max for the transcribed-rows list, omitting absent fields.
function fmtRange(p: Parameter): string {
  const v = p.value;
  if (!v) return "";
  const parts: string[] = [];
  if (v.min !== undefined) parts.push(`min ${v.min}`);
  if (v.typ !== undefined) parts.push(`typ ${v.typ}`);
  if (v.max !== undefined) parts.push(`max ${v.max}`);
  return parts.join(" / ");
}

function ParamEditor(props: { onAdd: (f: NewParamFields) => void }) {
  const [name, setName] = createSignal("");
  const [symbol, setSymbol] = createSignal("");
  const [limit, setLimit] = createSignal<LimitKind>(LimitKind.ABSOLUTE_MAX);
  const [min, setMin] = createSignal("");
  const [typ, setTyp] = createSignal("");
  const [max, setMax] = createSignal("");
  const [unit, setUnit] = createSignal("");
  const [cond, setCond] = createSignal("");

  const add = (): void => {
    if (!name().trim() && !symbol().trim()) return; // a row needs at least a name or a symbol
    props.onAdd({
      name: name().trim(),
      symbol: symbol().trim(),
      limitKind: limit(),
      min: numOrUndef(min()),
      typ: numOrUndef(typ()),
      max: numOrUndef(max()),
      unit: unit().trim(),
      condition: cond(),
    });
    setName(""); setSymbol(""); setMin(""); setTyp(""); setMax(""); setUnit(""); setCond("");
  };

  return (
    <div class="tx-editor">
      <label class="tx-field">Name<input value={name()} onInput={(e) => setName(e.currentTarget.value)} /></label>
      <label class="tx-field">Symbol<input placeholder="VIN" value={symbol()} onInput={(e) => setSymbol(e.currentTarget.value)} /></label>
      <label class="tx-field">Limit kind
        <select value={limit()} onChange={(e) => setLimit(Number(e.currentTarget.value) as LimitKind)}>
          <For each={LIMIT_LABELS}>{([v, l]) => <option value={v}>{l}</option>}</For>
        </select>
      </label>
      <div class="tx-range">
        <label class="tx-field">min<input value={min()} onInput={(e) => setMin(e.currentTarget.value)} /></label>
        <label class="tx-field">typ<input value={typ()} onInput={(e) => setTyp(e.currentTarget.value)} /></label>
        <label class="tx-field">max<input value={max()} onInput={(e) => setMax(e.currentTarget.value)} /></label>
        <label class="tx-field">unit<input placeholder="V" value={unit()} onInput={(e) => setUnit(e.currentTarget.value)} /></label>
      </div>
      <label class="tx-field">Condition<input placeholder="TA = 25°C (text only)" value={cond()} onInput={(e) => setCond(e.currentTarget.value)} /></label>
      <button class="tx-add" onClick={add}>Add parameter</button>
    </div>
  );
}

// PinEditor transcribes one pin of a pin table. The id defaults from the name (lowercased) because
// that is right almost always and the two are edited together; it stays editable because a part that
// prints one name on several terminals needs distinct ids for exactly those pins.
function PinEditor(props: { onAdd: (f: NewPinFields) => void }) {
  const [id, setId] = createSignal("");
  const [name, setName] = createSignal("");
  const [fn, setFn] = createSignal<PinFunction>(PinFunction.POWER_INPUT);
  const [desc, setDesc] = createSignal("");
  const [idTouched, setIdTouched] = createSignal(false);
  const effectiveId = (): string => (idTouched() ? id() : name().trim().toLowerCase().replace(/\s+/g, "_"));

  const add = (): void => {
    if (!name().trim() || !effectiveId()) return;
    props.onAdd({ id: effectiveId(), name: name().trim(), fn: fn(), description: desc().trim() });
    setId(""); setName(""); setDesc(""); setIdTouched(false);
  };

  return (
    <div class="tx-editor">
      <label class="tx-field">Pin name<input placeholder="VCCA" value={name()} onInput={(e) => setName(e.currentTarget.value)} /></label>
      <label class="tx-field">Id
        <input placeholder="vcca" value={effectiveId()} onInput={(e) => { setIdTouched(true); setId(e.currentTarget.value); }} />
      </label>
      <label class="tx-field">Function
        <select value={fn()} onChange={(e) => setFn(Number(e.currentTarget.value) as PinFunction)}>
          <For each={PIN_FUNCTIONS}>{([v, l]) => <option value={v}>{l}</option>}</For>
        </select>
      </label>
      <label class="tx-field">Description<input placeholder="A-port supply voltage…" value={desc()} onInput={(e) => setDesc(e.currentTarget.value)} /></label>
      <button class="tx-add" onClick={add}>Add pin</button>
    </div>
  );
}

// PackageList declares the bodies the part ships in. It sits with the datasheet metadata rather than
// under a region because a package is a property of the part, not of any one table.
function PackageList(props: { spec: () => PartSpec; onAdd: (id: string, name: string, suffix: string) => void; onDelete: (id: string) => void }) {
  const [id, setId] = createSignal("");
  const [name, setName] = createSignal("");
  const [suffix, setSuffix] = createSignal("");
  const add = (): void => {
    if (!id().trim()) return;
    props.onAdd(id(), name(), suffix());
    setId(""); setName(""); setSuffix("");
  };
  return (
    <div class="tx-packages">
      <h4>Packages</h4>
      <ul class="tx-pkg-list">
        <For each={props.spec().packages}>
          {(p) => (
            <li class="tx-row">
              <span class="tx-pkg-id">{p.id}</span>
              <span class="tx-pkg-name">{p.name}</span>
              <button class="tx-del" title="delete package" onClick={() => props.onDelete(p.id)}>×</button>
            </li>
          )}
        </For>
      </ul>
      <div class="tx-range">
        <label class="tx-field">id<input placeholder="pw" value={id()} onInput={(e) => setId(e.currentTarget.value)} /></label>
        <label class="tx-field">name<input placeholder="PW (TSSOP-14)" value={name()} onInput={(e) => setName(e.currentTarget.value)} /></label>
        <label class="tx-field">MPN suffix<input placeholder="PW" value={suffix()} onInput={(e) => setSuffix(e.currentTarget.value)} /></label>
      </div>
      <button class="tx-add" onClick={add}>Add package</button>
    </div>
  );
}

// TranscribePanel is the manual backend's editor: datasheet metadata (once), a region's routing
// type, and, for a table region, the parameter rows transcribed against it. A non-table type shows
// where its extraction lands (WS13-002/003) rather than a table editor, so the tag is captured but
// no wrong-shaped transcription is offered.
export function TranscribePanel(props: TranscribeHandlers) {
  return (
    <div class="tx-panel">
      <div class="tx-meta">
        <h3>Datasheet</h3>
        <label class="tx-field">MPN<input placeholder="LM1117" value={props.spec().mpn} onInput={(e) => props.setMeta({ mpn: e.currentTarget.value })} /></label>
        <label class="tx-field">Manufacturer<input value={props.spec().manufacturer} onInput={(e) => props.setMeta({ manufacturer: e.currentTarget.value })} /></label>
        <label class="tx-field">Device class<input placeholder="ldo" value={props.spec().deviceClass} onInput={(e) => props.setMeta({ deviceClass: e.currentTarget.value })} /></label>
        <PackageList spec={props.spec} onAdd={props.addPackage} onDelete={props.deletePackage} />
      </div>
      {/* The structural problems a save would be rejected for, shown while they are being created.
          Silent on a merely incomplete spec, which is what every spec under transcription is. */}
      <Show when={pinProblems(props.spec()).length}>
        <ul class="tx-problems">
          <For each={pinProblems(props.spec())}>{(msg) => <li>{msg}</li>}</For>
        </ul>
      </Show>
      <Show when={props.region()} fallback={<div class="tx-hint">Select a region, or drag on the page to draw one, to transcribe it.</div>}>
        {(r) => (
          <div class="tx-region">
            <h3 class="tx-region-head">
              Region {r().id}
              <Show when={props.deletableRegion()}>
                <button class="tx-del-region" title="delete this region" onClick={props.onDeleteRegion}>Delete region</button>
              </Show>
            </h3>
            <label class="tx-field">Type
              <select value={props.regionType()} onChange={(e) => props.setType(e.currentTarget.value as RegionType)}>
                <For each={REGION_TYPES}>{(t) => <option value={t}>{t}</option>}</For>
              </select>
            </label>
            <Show
              when={props.regionType() === "table" || props.regionType() === "pinout"}
              fallback={<div class="tx-hint">Transcription for “{props.regionType()}” lands with its backend (schematic → WS13-002, chart → WS13-003). Tagged here so coverage tracks it.</div>}
            >
              {/* A pin function table IS a table, so a table region offers both; a pinout drawing
                  yields pins only. */}
              <Show when={props.regionType() === "table"}>
                <ParamEditor onAdd={props.addParam} />
                <ul class="tx-params">
                  <For each={paramsForRegion(props.spec(), r().id)}>
                    {(p) => (
                      <li class="tx-row">
                        <span class="tx-p-name">{p.name || p.symbol}</span>
                        <span class="tx-p-val">{fmtRange(p)} {p.unit}</span>
                        <Show when={props.spec().pins.length}>
                          <span class="tx-binds">
                            <For each={props.spec().pins}>
                              {(pin) => {
                                // Reads through props.spec() rather than the captured p, so the
                                // chip TRACKS. A plain p.pinRefs read is not a signal, so Solid
                                // would wrap this in an effect with no dependencies: the binding
                                // would change in the data and never on screen.
                                const bound = (): boolean =>
                                  props.spec().parameters.some((x) => x === p && x.pinRefs.includes(pin.id));
                                return (
                                  <button
                                    class={`tx-bind ${bound() ? "on" : ""}`}
                                    title={bound() ? `unbind ${pin.name}` : `bind to ${pin.name}`}
                                    onClick={() => props.toggleBinding(p, pin.id)}
                                  >
                                    {pin.name}
                                  </button>
                                );
                              }}
                            </For>
                          </span>
                        </Show>
                        <button class="tx-del" title="delete" onClick={() => props.deleteParam(p)}>×</button>
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
              <h4 class="tx-pins-head">Pins</h4>
              <PinEditor onAdd={props.addPin} />
              <ul class="tx-pins">
                <For each={pinsForRegion(props.spec(), r().id)}>
                  {(pin) => (
                    <li class="tx-row">
                      <span class="tx-p-name">{pin.name}</span>
                      <span class="tx-pin-id">{pin.id}</span>
                      <span class="tx-nums">
                        <For each={props.spec().packages}>
                          {(pkg) => (
                            <label class="tx-num" title={`${pin.name} designator in ${pkg.name || pkg.id}`}>
                              {pkg.id}
                              <input
                                value={
                                  props.spec().pins.find((x) => x.id === pin.id)?.numbers
                                    .find((n) => n.packageRef === pkg.id)?.number ?? ""
                                }
                                onChange={(e) => props.setPinNumber(pin, pkg.id, e.currentTarget.value)}
                              />
                            </label>
                          )}
                        </For>
                      </span>
                      <button class="tx-del" title="delete pin" onClick={() => props.deletePin(pin)}>×</button>
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </div>
        )}
      </Show>
    </div>
  );
}
