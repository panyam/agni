// Entry point for the landing page ("/"), the third page beside the viewer's main.ts and the browse
// page.
//
// The page holds no state of its own. The two destinations are plain server-rendered links that work
// without JavaScript, and the two islands below are shortcuts past them, so a failure in either
// still leaves a usable page.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { projectsIsland, recentsIsland } from "./landingpanels.jsx";

class LandingRoot extends BaseComponent {
  override performLocalInit(): LCMComponent[] {
    const children: LCMComponent[] = [];
    const recentsEl = document.getElementById("landing-recents");
    const projectsEl = document.getElementById("landing-projects");
    if (recentsEl) {
      // One clock read for the whole render, so every row ages against the same instant.
      // Where the datasheets workbench is served, set by `agni serve --datasheets-url`; empty hides
      // datasheet recents, since the workbench is a separate service (agni issue 744).
      children.push(recentsIsland(recentsEl, this._eventBus, Date.now(), recentsEl.dataset.datasheetsUrl ?? ""));
    }
    if (projectsEl) children.push(projectsIsland(projectsEl, this._eventBus));
    return children;
  }
}

const bus = new EventBus();
const controller = new LifecycleController(bus);
void controller.initializeFromRoot(new LandingRoot("app", document.body, bus)).catch((err) => {
  console.error(err);
});
