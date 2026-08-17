import { For, Show, createSignal } from "solid-js";

// The sheet badge strip: which sheets an entity appears on, one chip each, click to go there.
//
// Three panels render it (query results, findings, diff changes) and it used to be a bare For loop
// in each. What forced it into one place was a design with 21 sheets: a ground net is on all of
// them, so the strip carried 21 chips. In the findings and diff panels that made a row five lines
// tall; in the query table, whose columns are fixed-width, the strip did not wrap at all and painted
// straight over the two columns to its right. Measured on that table: the last chip's right edge sat
// 1927px past the cell's own.
//
// So the strip shows the first few and counts the rest. A count is the honest summary anyway — past
// a handful, "which sheet" is a menu to open rather than a label to read, and the entity is on
// enough sheets that the interesting fact is HOW MANY.
//
// The cell still needs to be able to wrap and, failing that, to scroll (see .query-table td). A cap
// is what makes the common case one line; it is not what keeps the table honest.
const DEFAULT_LIMIT = 3;

// SheetBadges is generic over the badge so each panel keeps its own payload: the query and findings
// panels navigate by sheet id, the diff panel by the index of a sheet PAIR. Label, hover and click
// are supplied, which is the whole difference between the three call sites.
export function SheetBadges<T>(props: {
  items: T[];
  label: (b: T) => string;
  title: (b: T) => string;
  onSelect: (b: T) => void;
  limit?: number;
}) {
  const [expanded, setExpanded] = createSignal(false);
  const limit = (): number => props.limit ?? DEFAULT_LIMIT;
  const shown = (): T[] => (expanded() ? props.items : props.items.slice(0, limit()));
  const hidden = (): number => Math.max(0, props.items.length - limit());

  return (
    <>
      <For each={shown()}>
        {(b) => (
          <span
            class="sheet-badge"
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
              : `${hidden()} more: ${props.items.slice(limit()).map(props.label).join(", ")}`
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
