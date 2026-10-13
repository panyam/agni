// The Tables panel's presenter (agni issues 898 and 956). It shows the tables TableService.Tabulate
// projects from an answer the page already holds, a check run or a revision comparison, and saves
// any of them through TableService.Encode. A query answer is the query panel's to show, so it is no
// source here, though saveAnswer serves that panel's Save CSV.
//
// It lays nothing out and encodes nothing itself. The rows are Tabulate's and the order is the one
// Tabulate's order_by applies with each column's type (so R2 sorts before R10), and a saved file is
// Encode's bytes, which are `--format csv`'s. A table on screen and the csv the CLI writes for the
// same answer therefore agree row for row, which is C35's rule for every table agni emits.

import type { Client } from "@connectrpc/connect";
import { type Table, type TabulateRequest, TableEncoding, TableService } from "./gen/agni/v1/webapi/tables_pb.js";

export type TableClient = Client<typeof TableService>;

// TableAnswer is what Tabulate projects: the oneof of the request, as the generated type spells it.
export type TableAnswer = TabulateRequest["answer"];

// SavedTable is one encoded file for the host to hand the reader, the same shape a saved report is.
export interface SavedTable {
  filename: string;
  contentType: string;
  content: Uint8Array;
}

// TablesSourceKey names where the panel's tables come from. The comparison and the two sides' check
// runs exist only while a comparison is open.
export type TablesSourceKey = "checks" | "compare" | "compare-a" | "compare-b";

// CompareOptions are the two diff options a comparison's table reports on request, as `agni diff
// --rename-approx --include-equal` does.
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

// nextOrder is the order a click on a column header asks for: ascending first, descending on a second
// click of the same column, and ascending again on a third.
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

  // setCompareOpen says whether a comparison is open, which offers or withdraws its sources. Closing
  // it while the panel shows one of them clears the panel, since that answer is gone.
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
