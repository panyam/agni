// Entry point for the landing page ("/"), the fourth page beside the viewer's main.ts, the browse
// page, and the datasheets workbench.
//
// The page answers one question, "where am I going", and it holds no state of its own: the two
// destinations are plain server-rendered links that work with no JavaScript at all, and the two
// islands below are shortcuts past them. That is why a failure in either leaves a usable page.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { projectsIsland, recentsIsland } from "./landingpanels.jsx";

class LandingRoot extends BaseComponent {
  override performLocalInit(): LCMComponent[] {
    const children: LCMComponent[] = [];
    const recentsEl = document.getElementById("landing-recents");
    const projectsEl = document.getElementById("landing-projects");
    if (recentsEl) {
      // One clock read for the whole render, passed down, so every row on the page ages against the
      // same instant rather than each against its own.
      children.push(recentsIsland(recentsEl, this._eventBus, Date.now()));
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
