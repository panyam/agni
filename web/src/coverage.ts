// The interface-coverage panel's view-side types (WS9-041). The presenter fetches
// GetInterfaceCoverage on design load and pushes a CoverageState; the panel renders the
// per-interface signal matrix and emits an onLocate(net) intent when a signal is clicked, reusing
// the same locate path a query cell or finding does.
import type { GetInterfaceCoverageResponse } from "./gen/agni/v1/webapi/checks_pb.js";

// SignalState is one required signal's coverage, mirroring the server's SignalCoverage.state values
// (WS9-041): "present" | "missing" | "dangling" | "pullup_missing".
export interface SignalCoverageItem {
  name: string;
  net: string; // matched net name; "" when missing (nothing to locate)
  state: string;
}

// InterfaceCoverageItem is one detected interface's matrix: the profile name, its anchor net, and
// each required signal in profile order.
export interface InterfaceCoverageItem {
  profile: string;
  anchor: string;
  signals: SignalCoverageItem[];
}

export interface CoverageState {
  interfaces: InterfaceCoverageItem[];
}

export interface CoverageView {
  setState: (s: CoverageState) => void;
}

export function emptyCoverage(): CoverageState {
  return { interfaces: [] };
}

// coverageFromResponse maps the GetInterfaceCoverage wire response into the panel's view state.
export function coverageFromResponse(resp: GetInterfaceCoverageResponse): CoverageState {
  return {
    interfaces: resp.interfaces.map((ic) => ({
      profile: ic.profile,
      anchor: ic.anchorNet,
      signals: ic.signals.map((s) => ({ name: s.name, net: s.net, state: s.state })),
    })),
  };
}

// presentCount returns how many of an interface's signals are fully present, for a "4/6" summary.
export function presentCount(item: InterfaceCoverageItem): number {
  return item.signals.filter((s) => s.state === "present").length;
}
