// Entry point for the designs page (/designs/), where the designs a server holds are listed, made
// and changed (agni issue 1719). /designs/ lists them, and /designs/<id> shows one design's artifacts.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { designIsland, libraryIsland } from "./librarypanels.jsx";
import { libraryRoute } from "./libraryupload.js";

class LibraryRoot extends BaseComponent {
  override performLocalInit(): LCMComponent[] {
    const el = document.getElementById("library");
    if (!el) return [];
    const route = libraryRoute(window.location.pathname);
    if (route.kind === "design") {
      document.title = `${route.id} — Agni designs`;
      return [designIsland(el, this._eventBus, route.id)];
    }
    return [libraryIsland(el, this._eventBus)];
  }
}

const bus = new EventBus();
const controller = new LifecycleController(bus);
void controller.initializeFromRoot(new LibraryRoot("app", document.body, bus)).catch((err) => {
  console.error(err);
});
