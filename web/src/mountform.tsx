// The form that adds a GitHub mount, shared by the /mounts/ browse page (agni issue 1749) and the
// designs page's link form (agni issue 1727).

import { createSignal, For, Show } from "solid-js";
import { ConnectError, type Client } from "@connectrpc/connect";
import type { WorkspaceService } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import type { Connection } from "@panyam/agni-client/gen/agni/v1/webapi/connections_pb";

type WsClient = Client<typeof WorkspaceService>;

// errorText is the server's own message for a Connect error, which names the flag or the ref that
// was wrong, and the error's text otherwise.
const errorText = (e: unknown): string => (e instanceof ConnectError ? e.rawMessage : String(e));

// signInHref links to signInUrl and back to the page the person is on.
export function signInHref(signInUrl: string, here: string): string {
  return `${signInUrl}?next=${encodeURIComponent(here)}`;
}

// repoName reads owner/repo from what a person pastes: owner/repo, a github.com URL, or a
// github:// source, with or without .git.
export function repoName(s: string): string {
  return s
    .trim()
    .replace(/^https?:\/\/github\.com\//, "")
    .replace(/^github:\/\//, "")
    .replace(/\.git$/, "")
    .replace(/\/+$/, "");
}

// AddMountForm makes a runtime mount over a GitHub repo. The server downloads the repo at the ref
// before it answers, so the form says it is working and stays disabled until then. A refusal shows
// the server's own message, such as a ref GitHub does not know.
//
// The connection picker lists the connections the caller may use (agni issue 1724). With none and a
// server that signs people in with GitHub, the form links there (agni issue 1727). Signing in keeps
// the person's GitHub token as their own connection, so the picker lists it on the way back.
export function AddMountForm(props: { client: WsClient; connections: () => Promise<Connection[]>; signInUrl?: string; onAdded: (name: string) => void; open?: boolean }) {
  const [busy, setBusy] = createSignal(false);
  const [msg, setMsg] = createSignal("");
  const [conns, setConns] = createSignal<Connection[]>([]);
  void props
    .connections()
    .then(setConns)
    .catch(() => setConns([]));
  const submit = async (e: SubmitEvent): Promise<void> => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const field = (n: string) => ((form.elements.namedItem(n) as HTMLInputElement | null)?.value ?? "").trim();
    const name = field("name");
    const repo = repoName(field("repo"));
    setBusy(true);
    setMsg(`Downloading ${repo}…`);
    try {
      const connection = field("connection");
      await props.client.createMount({ name, source: `github://${repo}`, ref: field("ref"), ...(connection ? { connection } : {}) });
      setMsg("");
      form.reset();
      props.onAdded(name);
    } catch (err) {
      setMsg(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <details class="mount-add" data-testid="add-mount" open={props.open}>
      <summary>Add a GitHub mount</summary>
      <form onSubmit={(e) => void submit(e)}>
        <input name="name" placeholder="name" required aria-label="Mount name" />
        <input name="repo" placeholder="owner/repo" required aria-label="GitHub repo" />
        <input name="ref" placeholder="ref (default branch)" aria-label="Tag, branch or commit" />
        <Show when={conns().length > 0}>
          <select name="connection" aria-label="Connection for a private repo">
            <option value="">public repo, no connection</option>
            <For each={conns()}>{(c) => <option value={c.name}>{c.displayName || c.name.replace(/^connections\//, "")}</option>}</For>
          </select>
        </Show>
        <button type="submit" disabled={busy()}>
          Add
        </button>
      </form>
      <Show when={conns().length === 0 && props.signInUrl}>
        {(url) => (
          <p class="mount-add-signin">
            For a private repo, <a href={signInHref(url(), window.location.pathname)}>sign in with GitHub</a>, and the server reads it through your own account.
          </p>
        )}
      </Show>
      <Show when={msg()}>{(m) => <div class="mount-add-msg">{m()}</div>}</Show>
    </details>
  );
}
