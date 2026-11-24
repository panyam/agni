# mountset

This file covers how agni serves mounts: the folder, GitHub, bucket and SharePoint mounts goapplib's
registry holds, and the host paths the engine reads through.

- **A GitHub commit past the cap is a lazy mount** (agni 1877). goapplib's `WithLazyPastLimits` lists
  the tree and downloads nothing, and `Lazy(m)` reports it (a snapshot with no manifest). A commit
  under the cap is snapshotted whole, as before.
- **Read a mount's files through `Table.FS(m)`, never `os.DirFS(Root(m))`.** `Root` is "" for a lazy
  mount, since its view holds only what was read. `FS` lists from the tree and fetches one file on
  open. Discovery (`liveTrees`) and the config resolver go through it.
- **`Resolve` fetches before it answers on a lazy mount.** A file comes with the folder it sits in,
  since a KiCad schematic opens its sheets off disk, or alone when that folder passes the cap. A
  folder past the cap is `ErrResourceExhausted`, and `Err` names `--mount-max-bytes`.
- **Don't build a lazy mount's whole view.** `HostPath` on its root fetches the repo the mount was made
  lazy to avoid, so `CreateMount` and startup skip it.
- **agnids sees only what agni serve fetched** from a lazy mount, since it opens the uploads
  read-only (#1880).
- **`Table.CanRead` is the one place a grant is checked** (agni 1889, 1896). It hands goapplib the
  caller's subject and `Caller.Claims`. The claims are `oidc:group:` and `github:org:` values that
  sign-in wrote into the token's scopes, and `authgate.Claims` drops any other scope. agnids passes
  grants only with `--connections-key`, since it builds no connections service without one.
