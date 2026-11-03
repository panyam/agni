// The Tables panel's presenter (agni issues 898 and 956). The page already holds an answer, a check
// run or a revision comparison. TableService.Tabulate projects tables from that answer, and the
// panel shows them and saves any of them through TableService.Encode. The query panel shows a query
// answer, so a query answer is no source here, though saveAnswer serves that panel's Save CSV.
//
// It lays nothing out and encodes nothing itself. Tabulate supplies the rows. Tabulate's order_by
// sorts them by each column's type, so R2 sorts before R10. A saved file holds Encode's bytes, and
// those match `--format csv`'s. So a table on screen and the csv the CLI writes for the same answer
// agree row for row. C35 asks that of every table agni emits.

import type { Client } from "@connectrpc/connect";
import { type Table, type TabulateRequest, TableEncoding, TableService } from "./gen/agni/v1/webapi/tables_pb.js";

export type TableClient = Client<typeof TableService>;

// TableAnswer is the answer oneof of a Tabulate request, as the generated type spells it.
export type TableAnswer = TabulateRequest["answer"];

// SavedTable is one encoded file for the host to hand the reader. It has the same shape as a saved
// report.
export interface SavedTable {
  filename: string;
  contentType: string;
  content: Uint8Array;
}

// TablesSourceKey names where the panel's tables come from. The comparison and the two sides' check
// runs exist only while a comparison is open.
export type TablesSourceKey = "checks" | "compare" | "compare-a" | "compare-b";

// CompareOptions holds two diff options. A comparison's table reports near-renames and equal entries
// only when asked, as `agni diff --rename-approx --include-equal` does.
export interface CompareOptions {
  nearRenames: boolean;
  equal: boolean;
}

// TablesState is what the panel renders. tables is empty until a source is shown, and active indexes
// it. orderBy is the order Tabulate applied, one column name with a leading "-" for descending.
export interface TablesState {
  source: TablesSourceKey | "";
  label: string;
  tables: Table[];
  active: number;
  orderBy: string;
  loading: boolean;
  error: string;
  compareOpen: boolean;
  compare: CompareOptions;
  note: string;
}

export interface TablesView {
  setState: (s: TablesState) => void;
}

export function emptyTables(): TablesState {
  return {
    source: "",
    label: "",
    tables: [],
    active: 0,
    orderBy: "",
    loading: false,
    error: "",
    compareOpen: false,
    compare: { nearRenames: false, equal: false },
    note: "",
  };
}

// TableAnswers is where each source's answer comes from. The composition root implements it over the
// presenters that hold the answers, so this presenter reads nothing itself. Each returns undefined
// with a reason when there is no answer to show yet, such as a run that has not happened.
export interface TableAnswers {
  answer(source: TablesSourceKey, compare: CompareOptions): Promise<{ answer?: TableAnswer; label: string; stem: string; reason?: string }>;
}

// nextOrder returns the order for a click on a column header. The first click asks for ascending, a
// second click on the same column for descending, and a third for ascending again.
export function nextOrder(current: string, column: string): string {
  return current === column ? `-${column}` : column;
}

export class TablesPresenter {
  private state: TablesState = emptyTables();
  private answer: TableAnswer | undefined;
  private stem = "";

  constructor(
    private readonly client: TableClient,
    private readonly view: TablesView,
    private readonly answers: TableAnswers,
    private readonly download?: (file: SavedTable) => void,
  ) {}

  private push(patch: Partial<TablesState>): void {
    this.state = { ...this.state, ...patch };
    this.view.setState(this.state);
  }

  // setNearRenames sets the comparison tables' near-rename pairing without asking for anything. So a
  // diff link that paired near-renames opens tables that pair them too (agni issue 1126).
  setNearRenames(on: boolean): void {
    this.push({ compare: { ...this.state.compare, nearRenames: on } });
  }

  // setCompareOpen says whether a comparison is open, and so offers or withdraws its sources. If the
  // panel shows one of those sources when the comparison closes, the panel clears, since that answer
  // is gone.
  setCompareOpen(open: boolean): void {
    const shown = this.state.source.startsWith("compare");
    if (!open && shown) {
      this.answer = undefined;
      this.push({ ...emptyTables(), compare: this.state.compare });
      return;
    }
    this.push({ compareOpen: open });
  }

  // show asks Tabulate for a source's tables and shows the first. A source with no answer yet says why
  // in the panel rather than showing an empty table.
  async show(source: TablesSourceKey, compare: CompareOptions = this.state.compare): Promise<void> {
    this.push({ source, loading: true, error: "", note: "", compare, orderBy: "" });
    try {
      const got = await this.answers.answer(source, compare);
      if (!got.answer) {
        this.answer = undefined;
        this.push({ label: got.label, tables: [], active: 0, loading: false, error: got.reason ?? "Nothing to show yet." });
        return;
      }
      this.answer = got.answer;
      this.stem = got.stem;
      const resp = await this.client.tabulate({ answer: got.answer });
      this.push({ label: got.label, tables: resp.tables, active: 0, loading: false });
    } catch (e) {
      this.push({ tables: [], loading: false, error: e instanceof Error ? e.message : String(e) });
    }
  }

  // select shows another of the answer's tables, in the order the panel already asked for.
  select(i: number): void {
    if (i >= 0 && i < this.state.tables.length) this.push({ active: i });
  }

  // sortBy asks Tabulate again for the same answer ordered by a column, so the order is the engine's
  // and the saved csv is in the order on screen.
  async sortBy(column: string): Promise<void> {
    if (!this.answer) return;
    const orderBy = nextOrder(this.state.orderBy, column);
    this.push({ loading: true });
    try {
      const resp = await this.client.tabulate({ answer: this.answer, orderBy: [orderBy] });
      this.push({ tables: resp.tables, orderBy, loading: false });
    } catch (e) {
      this.push({ loading: false, error: e instanceof Error ? e.message : String(e) });
    }
  }

  // saveActive writes the table on screen as csv, in the order on screen.
  async saveActive(): Promise<void> {
    const t = this.state.tables[this.state.active];
    if (!t) return;
    await this.encode([t], this.stem);
  }

  // saveAnswer tabulates an answer and saves the named tables of it (all of them when names is empty),
  // for the checks and query panels' Save CSV, which save without showing anything here.
  async saveAnswer(answer: TableAnswer, stem: string, names: string[] = []): Promise<string> {
    try {
      const resp = await this.client.tabulate({ answer });
      const tables = names.length ? resp.tables.filter((t) => names.includes(t.name)) : resp.tables;
      await this.encode(tables, stem);
      return "";
    } catch (e) {
      return `Could not save the csv: ${e instanceof Error ? e.message : String(e)}`;
    }
  }

  private async encode(tables: Table[], stem: string): Promise<void> {
    if (!this.download || tables.length === 0) return;
    const resp = await this.client.encode({ tables, encoding: TableEncoding.CSV, stem });
    for (const f of resp.files) this.download({ filename: f.filename, contentType: f.contentType, content: f.content });
  }
}
