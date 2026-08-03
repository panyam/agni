import { createSignal, For, Show } from "solid-js";
import { LimitKind, type PartSpec, type Parameter } from "./gen/agni/v1/param/param_pb.js";
import { REGION_TYPES, type Region, type RegionType } from "./regions.js";
import { paramsForRegion, type NewParamFields } from "./bank.js";

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
      </div>
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
              when={props.regionType() === "table"}
              fallback={<div class="tx-hint">Transcription for “{props.regionType()}” lands with its backend (schematic → WS13-002, chart → WS13-003). Tagged here so coverage tracks it.</div>}
            >
              <ParamEditor onAdd={props.addParam} />
              <ul class="tx-params">
                <For each={paramsForRegion(props.spec(), r().id)}>
                  {(p) => (
                    <li class="tx-row">
                      <span class="tx-p-name">{p.name || p.symbol}</span>
                      <span class="tx-p-val">{fmtRange(p)} {p.unit}</span>
                      <button class="tx-del" title="delete" onClick={() => props.deleteParam(p)}>×</button>
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
