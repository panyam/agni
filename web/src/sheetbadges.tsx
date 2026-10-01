import { For, Show, createSignal } from "solid-js";

// The sheet badge strip shows which sheets an entity appears on, one chip each, click to go there.
// The query results, findings and diff changes panels all render it.
//
// It shows the first few chips and counts the rest (#337). A ground net on a 21-sheet design carried
// 21 chips, which made a findings row five lines tall and, in the fixed-width query table, painted
// the last chip 1927px past its cell. Past a handful of sheets, HOW MANY is the useful fact.
//
// The cap only keeps the common case to one line. The cell still has to wrap and, failing that,
// scroll (see .query-table td).
const DEFAULT_LIMIT = 3;

// SheetBadges is generic over the badge so each panel keeps its own payload. The query and findings
// panels navigate by sheet id, the diff panel by the index of a sheet PAIR, and each supplies label,
// hover and click.
// `active` marks the badge whose sheet is on screen, so a reader who jumped to a sheet can still see
// where they jumped FROM (#341). It is a predicate over the viewer's actual state rather than an
// index of the last click here, so navigating by the sheet tabs or the drawing moves the mark instead
// of stranding it. A caller that passes none marks nothing, as the findings and diff strips do.
export function SheetBadges<T>(props: {
  items: T[];
  label: (b: T) => string;
  title: (b: T) => string;
  onSelect: (b: T) => void;
  active?: (b: T) => boolean;
  limit?: number;
}) {
  const [expanded, setExpanded] = createSignal(false);
  const limit = (): number => props.limit ?? DEFAULT_LIMIT;
  // The cut grows to include an active badge past the cap. Hidden, the strip would look unmarked and
  // the reader would conclude they are somewhere else.
  const cut = (): number => {
    const base = limit();
    if (!props.active) return base;
    const i = props.items.findIndex((b) => props.active!(b));
    return i < base ? base : i + 1;
  };
  const shown = (): T[] => (expanded() ? props.items : props.items.slice(0, cut()));
  const hidden = (): number => Math.max(0, props.items.length - cut());

  return (
    <>
      <For each={shown()}>
        {(b) => (
          <span
            class={`sheet-badge${props.active?.(b) ? " on" : ""}`}
            title={props.title(b)}
            onClick={(e) => {
              e.stopPropagation();
              props.onSelect(b);
            }}
          >
            {props.label(b)}
          </span>
        )}
      </For>
      <Show when={hidden() > 0}>
        <span
          class="sheet-badge sheet-badge-more"
          title={
            expanded()
              ? "show fewer sheets"
              : `${hidden()} more: ${props.items.slice(cut()).map(props.label).join(", ")}`
          }
          onClick={(e) => {
            e.stopPropagation();
            setExpanded(!expanded());
          }}
        >
          {expanded() ? "−" : `+${hidden()}`}
        </span>
      </Show>
    </>
  );
}
