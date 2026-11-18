// Entry point for the store's pages (agni issues 1719, 1697). /designs/ lists the designs a server
// holds and /designs/<id> shows one, /projects/ and /projects/<id> the projects, and /packs/ and
// /packs/<id> the packs. The server serves one shell for all six, and this reads which from the URL.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { designIsland, libraryIsland } from "./librarypanels.jsx";
import { packIsland, packsIsland, projectIsland, projectsIsland } from "./storepanels.jsx";
import { storeRoute } from "./storepages.js";

class LibraryRoot extends BaseComponent {
  override performLocalInit(): LCMComponent[] {
    const el = document.getElementById("library");
    if (!el) return [];
    const route = storeRoute(window.location.pathname);
    switch (route.kind) {
      case "design":
        document.title = `${route.id} — Agni designs`;
        return [designIsland(el, this._eventBus, route.id)];
      case "projects":
        document.title = "Agni projects";
        return [projectsIsland(el, this._eventBus)];
      case "project":
        document.title = `${route.id} — Agni projects`;
        return [projectIsland(el, this._eventBus, route.id)];
      case "packs":
        document.title = "Agni packs";
        return [packsIsland(el, this._eventBus)];
      case "pack":
        document.title = `${route.id} — Agni packs`;
        return [packIsland(el, this._eventBus, route.id)];
      default:
        return [libraryIsland(el, this._eventBus)];
    }
  }
}

const bus = new EventBus();
const controller = new LifecycleController(bus);
void controller.initializeFromRoot(new LibraryRoot("app", document.body, bus)).catch((err) => {
  console.error(err);
});
