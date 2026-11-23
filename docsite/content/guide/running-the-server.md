---
title: "Running the server"
description: "Bring up the web viewer on your own machine with Docker, and bring your designs with you."
---

The [viewer](../../architecture/web-app/) is a server you run, not a hosted service. This page is
about bringing it up on your own machine so a team can open a browser and work, rather than each
engineer installing a toolchain.

If you only want the CLI, [Getting started](../getting-started/) is the shorter path.

## The one-liner

```
docker run -p 8080:8080 -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.1
```

Open `http://localhost:8080`. Your boards are in the file tree on the left, under a mount named
`boards`.

With no `-v` at all it still runs, serving the two demo boards baked into the image, so you can
confirm the thing works before pointing it at your own designs.

The examples on this page pin a version, as
[Getting started](../getting-started/#install) does, because a report is only reproducible if you can say
which build produced it. Every published version is on
[the package page](https://github.com/panyam/agni/pkgs/container/agni), and `:latest` tracks the
newest if you would rather not pin. `agni version` inside the container always tells you what you
actually have.

## Bringing your designs

Every subdirectory of `/workspace` becomes a mount named after itself. One `-v` per folder, no
flags:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v ~/datasheets:/workspace/datasheets \
  ghcr.io/panyam/agni:v0.1.1
```

That gives two mounts, `boards` and `datasheets`. Each mount is a containment boundary, because the
browser addresses files as a mount name plus a mount-relative path, never an absolute one, and the
server rejects any path that escapes its mount. A symbolic link inside a mount is followed when it
points at a file or folder in the same mount, written as a relative path. A link that leads out of
the mount, or names its target by an absolute path, reads as missing (agni issue 1759).

{{ includeFile "figures/mount-topology.svg" }}

Dotfile directories are skipped, so a `.git` sitting in a bind-mounted parent does not show up as a
design folder.

## Without Docker

The image carries the viewer's built assets and the symbol libraries, and a server needs both
beyond the engine. If you run the binary directly, you supply the viewer's built assets yourself.

```
agni serve --addr :8080 --mount boards=~/boards --web-dir /path/to/agni/web
```

`--web-dir` is the directory holding the viewer's own `templates/` and its built `static/*.js`, not a
folder of designs. It defaults to `web`. A repo checkout keeps them there, so from a checkout you can
leave it off.

With no web dir at all, a release binary finds its own viewer. A `go install` of a release tag
downloads the bundle its release attached. A build that is not a release, such as `make agni` or
`go install ...@main`, has no bundle, so `agni serve` serves the API alone. The run says so on
stderr, and `/` answers with a short text page listing the services and how to get the viewer. The
Connect API, `/healthz`, and the rule and relation doc images all work. A script or the
[Python client](../../build/other-languages/) needs nothing more.

That only happens when NOTHING named a web dir and there is no `./web`. If you name a directory
through the flag, an `agni.yaml` or the environment, it still has to exist and hold a built viewer.
A `./web` that holds templates without its bundle also stops the run. A checkout that has not run
`make ui` is in that state. Either one is a mistake you want to hear about.

The datasheets workbench is a separate service, `agnids`, built from the `datasheet/` module. It has
its own mounts and holds the docling environment, so it deploys apart from the viewer. It only reads
its mounts. The doc-IR it extracts and the annotations people save go in its corpus store, keyed by
each PDF's hash, so `--corpus` names where they are kept:

```
agnids serve --addr :8090 --mount ds=~/datasheets --corpus ~/corpus --web-dir /path/to/agni/datasheet/web --viewer-url http://host:8080
```

Its web assets are its own, `datasheet/web`, built by `make ui`, so a workbench deployment needs no
part of the viewer's `web/`. From a checkout, `make dsserve DATASHEET_DIR=~/datasheets` runs it. The
two link to each other only when told where the other is: `agni serve --datasheets-url
http://host:8090` shows the landing page's Datasheets card, and `agnids serve --viewer-url
http://host:8080` makes the workbench's heading link home. The same service can also serve a
published corpus of PartSpecs with `--corpus <dir>`. `agni serve --params-url http://host:8090`
reads that corpus in place of a `--params` directory (see
[Datasheets](../datasheets/#serving-a-shared-corpus)).

Two ways to avoid typing it every time:

| where you put it | what it suits |
|---|---|
| `web_dir:` in an `agni.yaml` ([machine configuration](../cli-reference/#machine-configuration-in-agniyaml)) | per-directory, and it travels with a checkout |
| `AGNI_WEB_DIR` in the environment | an installed binary whose assets sit at a fixed path |

A run says on stderr when the value came from the environment, because an `AGNI_WEB_DIR` exported
months ago outlives the memory of exporting it.

**For one design, reach for [`agni open`](../cli-reference/#open-design) instead.** It picks a free
port and prints the URL of the board rather than of a file browser. It finds the viewer the same way
`serve` does, and unlike `serve` it needs one, since showing a page is its whole job. `agni serve` is for several designs at once, or for a server other people reach.

## Symbol libraries are already there

Some schematics name their symbols rather than embedding them. Such a schematic needs the symbol
library before it can resolve to a pin-level {{ explainable "netlist" }}. The image ships the
KiCad, xschem, and gEDA symbol libraries and points `--symbol-path` at all three, so external
symbols resolve with nothing to install.

Without the libraries the design fails quietly. It still reads, but it reads short, with fewer
components, fewer nets, and therefore fewer findings, and no error tells you so. If your component
or net counts look low, check this first, whether you are running in Docker or not.

The libraries are most of the image's ~340MB. The size is deliberate, because a smaller
image that renders your board wrong is the worse default.

## Running one-shot commands

The image's entrypoint is the `agni` binary and the server is only its default command, so any
subcommand works with the same environment the server has:

```
docker run -v ~/boards:/workspace/boards ghcr.io/panyam/agni:v0.1.1 \
  check /workspace/boards/board.kicad_pro --format json
```

This is mostly for CI, where a container is easier than provisioning a Go toolchain, and where it
matters that the gate and the viewer run identical engine and symbol data. At your own desk,
installing the CLI is nicer. See [the CLI reference](../cli-reference/).

## Asking a running server from the CLI

Every `agni` command reads the design itself, so a second command on the same board reads and
projects it again. A running server keeps what it read (see [Kept reads](#kept-reads)), and
`--remote` on `query`, `check`, `trace` and `review` sends the command's request there instead:

```
agni query designs/board --remote http://localhost:8080 'component.mpn(?r, ?m) => count(?r)'
```

The output matches the local command's, because the CLI already builds a request for the server
and renders the response it gets back. On the Jetson board a warm query takes 0.02 s
end to end against 0.33 s run locally, and a check 0.18 s against 1.39 s.

The design's files go with the request, so the server needs no copy of them (agni issue 1061). The
CLI sends a manifest of the files its analysis reads. Those are the design's folder, its project's
descriptor and the config directories the project names, and `ListDesignFiles` names the same set
for a browser. The server asks for the contents it does not already hold, so a second run of an
unchanged design sends the manifest alone. A server started with `--upload-max-bytes 0` takes no
files. Otherwise it keeps what it was sent up to that limit, 1 GB by default.
The server answers with its own datasheet corpus and profiles, the ones it was started with. So
`--params` and `--profile-path` are refused with `--remote` rather than ignored. So is `--timing`,
since it would time this process. The server's own `--slow-request` and the `Agni-Timing` header time
the request where it runs. The server scores and returns a review without storing it, so the review
leaves nothing in the server's list. `stats`, `diff`, `render` and the
other commands that read a design themselves still run locally.

## Health

`GET /healthz` returns 200 while the server is up. The image declares a `HEALTHCHECK` that calls
`agni healthcheck` against it, so an orchestrator gets liveness with no extra tooling in the image.

The probe reports that the process is serving HTTP and nothing more. Mounts, the rule catalog, and
the parameter set are all validated at startup, so a misconfigured server fails before it ever
listens. A 200 is not evidence that your `--profile-path` was right.

## House rules for the whole server

The catalog overlay is deployment configuration, applied once at startup to every rule-running
surface:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v ~/houserules:/etc/agni \
  ghcr.io/panyam/agni:v0.1.1 \
  serve --addr :8080 --mount-root /workspace \
        --profile-path /etc/agni/profiles \
        --conventions /etc/agni/conventions.yaml
```

Overriding the command like this replaces the default arguments, so `--mount-root` has to be
repeated. The symbol paths do not, because they come from the environment.

A profile named after a built-in supersedes the built-in for **every** design this server reads,
so point it at rules that suit the whole mounted set rather than one
project. See [interface profiles](../interface-profiles/) and
[naming conventions](../naming-conventions/).

## The store and the uploads

`serve --store <url>` names where the server keeps its records, and `serve --uploads <url>` names
where it keeps the files of designs its users upload (agni issues 1732, 1739). `--uploads` takes a
`file://` URL naming a folder, created if absent, such as `--uploads file:///srv/agni/files`, or a
bucket, `s3://<bucket>/<prefix>` or `gs://<bucket>/<prefix>` (agni issue 1730). A bucket holds every
file, and `--uploads-cache` names a local folder in front of it. The folder keeps a copy of each
file the server reads and every view the engine reads, since the engine reads a design through a
folder. A server that loses the folder, as a pod on a fresh node does, fetches each file from the
bucket when it needs it and rebuilds the views at start. The bucket signs in with the server's own
credentials, the AWS default chain or Application Default Credentials, and `--s3-endpoint` or
`--gcs-endpoint` points it at another host, such as MinIO.
`--store` takes one of four kinds of URL (agni issue 1733). A bare path is refused with the `file://`
form it meant.

- `--store file:///srv/agni/meta` keeps each record as a small YAML or JSON file in that folder.
- `--store sqlite:///srv/agni/agni.db` keeps every record as a row of one sqlite database file. The
  file and its folder are created if absent, and the file is readable by the server's user alone.
- `--store postgres://agni@db.internal:5432/agni?sslmode=require` keeps them as rows in a postgres
  database, which must exist. The password comes from `PGPASSWORD` or a passfile (`PGPASSFILE`, or
  `~/.pgpass`), and a URL with a password in it is refused, since it would show in `ps`. Options
  such as `sslmode` and `search_path` pass through to the driver.
- `--store datastore://my-project?namespace=agni` keeps them as entities in Cloud Datastore, or
  Firestore in Datastore mode, in that namespace. It signs in with Application Default Credentials,
  and `datastore://?namespace=agni` takes the project from them. `DATASTORE_EMULATOR_HOST` points it
  at the emulator.

In each, runtime mounts, connections, people's sealed tokens and the `--auth` tokens go into the same
store. `agni token` and `agnids serve --store` take the same URL. sqlite and postgres go through
GORM, and the server creates the tables it needs on start. The records are the same in all four, so
the rest of this page applies to any of them. A server doesn't move records from one kind of store to
another, so pick one when you first start it.

The store keeps records and no file bytes. It holds each stored design's record, with every
artifact's paths and hashes, the projects, each snapshot's definition and the review runs. The
uploads keep each file once, under its SHA-256, and never change a file once written.

A store alone is enough to keep snapshots and review runs. A design's files need the uploads too,
and a server without `--uploads` refuses them. `--uploads` without `--store` is refused, since
nothing would record what the uploads hold.

## Keeping snapshots across a restart

A client asks its questions about a snapshot, a design the server ingested once and holds under a
name. The server keeps each snapshot's definition, so a question on one it evicted is answered after
it ingests the snapshot again. With `--store`, those definitions go on disk, so a question on a
snapshot made before a restart answers too (agni issue 1639). Without it they live in memory.
Uploaded file contents go to `--uploads`, so a snapshot sent as files also comes back.

## Keeping designs

With `--store` and `--uploads`, the server keeps designs and projects of its own (agni issues 1693,
1694, 1696, 1739).

Say a user keeps a design `memory_unit` from a KiCad folder:

```bash
agni designs push ./memory_unit --series b200 --remote http://localhost:8080
agni designs list --series b200 --remote http://localhost:8080
agni designs put memory_unit schematic ./memory_unit-fixed/ --remote http://localhost:8080
```

`push` puts the folder's schematic, with its sheets, and its board as two artifacts of the design.
`put` later replaces the schematic, every sheet in one write, and leaves the board alone. The
`check`, `diff` and `query` commands then name the stored design instead of a folder, and the server
reads its own copy (agni issue 1695):

```bash
agni check designs/memory_unit --remote http://localhost:8080
agni diff designs/memory_unit designs/memory_unit-rev-b --remote http://localhost:8080 --format json
```

A stored design keeps the `intent:` of the folder's `design.yaml`. The `entry:` and `companions:`
decide which files become its artifacts and which one analysis reads, so an ODB++ board and an EDIF
schematic export declared together stay one design. Nothing else from the descriptor is kept. The
project's convention, checklists, profiles and params go up as a pack, `packs/<id>`, which `agni
packs push` reads from a `project.yaml` (agni issue 1704). A check names the pack, and the server
reads the design under it:

```bash
agni packs push ./b200 --remote http://localhost:8080
agni checklists packs/b200 --remote http://localhost:8080
agni check designs/memory_unit --pack b200 --remote http://localhost:8080
agni review designs/memory_unit --pack b200 --remote http://localhost:8080
```

`review` takes its checklist from the pack, the project's first one unless `--checklist` names
another. `agni checklists packs/<id>` lists them, the default first, and a design on the server has
none of its own to list. When the folder's `design.yaml` extends the project, `agni designs push` keeps the project
as the pack itself, under its `name:`, and says the `--pack` to use. A pack the server holds with the
same hash is left alone, and `--no-pack` skips it.

A report keeps such a check under the design, and says later whether the design's files or the pack
changed since it ran:

```bash
agni reports create designs/memory_unit --pack b200 --remote http://localhost:8080
agni reports list designs/memory_unit --remote http://localhost:8080
```

`push` takes the pack's id from the project's `name:`, so its profile rules are named
`b200-profiles/...` as they are on the folder. `--pack` repeats, and each pack layers over the one
before it. Under `--pack` the server's catalog decides the rules, so `--rule` names go as written and
`--tag` is refused.

A snapshot can name its design the same way, `Snapshot.design`. The id `DesignSnapshotID` derives
from the name lets every client that opens the design share one snapshot, and a server started with
`--store` rebuilds it after a restart by looking the name up again.

The store keeps the design's record as `designs/memory_unit.yaml`. The uploads keep its files under
`blobs/`, and serve them as the mount `uploads`, at `designs/memory_unit/<rev>/<artifact>/<path>`.
Each write makes a new revision's folder, named by a hash of what it holds, and never changes an old
one. So a snapshot read from the older revision keeps answering. That folder also holds a
`design.yaml` the server writes from the record, so `agni check` on it reads it like any other
design folder. At startup, and every hour while it runs, the server removes revisions and files
that no record or snapshot names and that are over an hour old (agni issue 1742).

A project is a title and a list of design names, kept as `projects/<id>.json` in the store, so one
design can sit in several projects. `agni projects create` and `agni projects add` make one and list
designs in it.

The viewer's designs page, `/designs/`, does the same from a browser (agni issue 1719). It lists
every design the server holds, by series with each series' latest marked, and a form there makes a
design from a dropped folder. A design's page, `/designs/<id>`, lists its artifacts and replaces or
deletes one. The folder browser
is at `/mounts/`, and an older `/designs/<mount>/<path>/view` link redirects there.

`DesignService` and `ProjectService` create, change and delete them over the wire. A client sends an
artifact as a manifest of paths and hashes, and sends any contents the server asks for with
`SnapshotService.UploadBlobs`, as `--remote` already does. A project in a mounted folder stays
read-only, and a write to it is refused.

A folder in a mount is not a design until you add it (agni issue 1736). `add` lists the boards it
finds, and keeps the ones you name:

```bash
agni designs add boards/memory_unit --remote http://localhost:8080
agni designs add boards/memory_unit --yes --series b200 --remote http://localhost:8080
```

The server copies each board's files, and each artifact records the mount and folder they came
from.

Some schematics keep their parts' pins in separate symbol files: gEDA, Lepton and xschem read `.sym`
files from the folders a `design.yaml` names under `symbols:` (agni issue 1756). An added design
keeps each of those folders as an artifact of its own, a symbol library, even when the folder sits
beside the board's rather than inside it. So the design reads alone. When the `design.yaml` extends
a project, the server keeps that project as a pack, as `agni designs push` does, and the design
reads under it. So the project's own symbol folders, profiles and conventions are shared by every
design added from it, held once. `agni check designs/<id>` with no `--pack` reads under the design's
packs, and says so. The files themselves are stored once by hash, however many designs copy them.
`--no-symbol-copy` leaves the design's own symbol folders out, for a library a pack already holds,
and names each one it left out.

In a container, give the store and the uploads a volume. The image makes `/var/lib/agni/meta` and
`/var/lib/agni/files` owned by its user, so a named volume on either starts out writable. The
compose bundle (`deploy/compose.yaml`) mounts one volume, `data`, on `/var/lib/agni` and keeps both
in it (agni issue 1698).

## Mounts made at runtime

A server with `--store` and `--uploads` can mount a GitHub repo while it runs (agni issue 1735). Say
the team keeps a board in a public repo and wants it in the browser without a clone on the server.
`agni mounts add` takes a name, a `github://<owner>/<repo>` source and a ref. The server asks GitHub
which commit the ref names, downloads that commit's tarball, and serves it as a read-only mount:

```
agni mounts add jetson github://antmicro/jetson-agx-thor-baseboard --ref main --remote http://localhost:8080
agni mounts ls --remote http://localhost:8080
agni mounts rm jetson --remote http://localhost:8080
```

The `/mounts/` page does the same (agni issue 1749). On a server that can make mounts, "Add a GitHub
mount" under the folders takes a name, a repo as `owner/repo` or its GitHub URL, and a ref. Each
mount made this way has a × that removes it after a confirm. A mount from the flags has none. Both
are calls to `WorkspaceService.CreateMount` and `DeleteMount`, which any client can make.

The mount is a snapshot. It serves the commit it resolved to until it is deleted and made again, so
a push to the branch does not change what a stored design or a report pointed at. The snapshot's
files go to the uploads once each, by hash, beside the designs' files. Its view is
`mounts/<name>/<commit>/` in the uploads, and the store keeps one record, `mounts/<name>.yaml`, with
every file's path and hash. A restart reads the records back, and rebuilds a view the uploads lost
from its files. A snapshot over 256 MB is refused.

A repo that keeps its schematics or board files in Git LFS reads as the files themselves (agni issue
1809). Git stores a short pointer in place of each LFS file. The server swaps each pointer for its
object through the repo's LFS batch API, with the mount's connection, and checks the object's size
and SHA-256. That happens for a mount, a linked artifact and a read at another commit alike. On
GitHub Enterprise Server the LFS host is the `--github-api` URL without `/api/v3`. A pointer whose
object can't be read fails the mount or the link with FAILED_PRECONDITION, naming the file, so a
design never reads short with no error. Typical causes are no LFS permission, a used-up quota or a
missing object. A `--mount` folder refuses a pointer the same way, so a checkout made without
`git lfs pull` fails loudly too.

`DeleteMount` forgets a mount `CreateMount` made. A mount from `--mount` or `--mount-root` comes from
the server's flags, and deleting one is refused. So is deleting a mount that an artifact is linked
to, as the next section describes. A runtime mount can't name a folder on the server's disk, since a
client must not choose which folder is served. `--mount` names a folder. A private repo reads through
a connection, as "Connections" below describes.

### Buckets and SharePoint

A runtime mount can also come from an S3 bucket, a Cloud Storage bucket, or a SharePoint or OneDrive
folder (agni issues 1729, 1730). Say the team drops its exports into `s3://acme-boards/jetson`, or
keeps them in a SharePoint document library. The server lists the folder, downloads every file into
the uploads, and serves it from a view as it does a repo:

```bash
agni mounts add boards s3://acme-boards/jetson --connection aws --remote http://localhost:8080
agni mounts add exports gs://acme-exports/hw --connection gcs --remote http://localhost:8080
agni mounts add sp msgraph://sites/<site-id>/drives/<drive-id>/root/hw --connection sp --remote http://localhost:8080
```

A bucket or a drive has no refs, so `--ref` stays empty. The snapshot pins each file at the version
it had: an S3 or Cloud Storage object version when the bucket keeps them, else its ETag, and a
SharePoint item's content tag. An object overwritten later leaves the mount as it was. A link that
follows the folder moves on the next `designs sync`, or on its own with `--poll` or a Graph
notification. A public bucket needs no connection, and Microsoft Graph always does. `--s3-endpoint`
points the S3 mounts at an S3-compatible store such as MinIO, `--gcs-endpoint` does the same for
Cloud Storage, and `--graph-api` names a national cloud's Graph.

On a shared server every caller the server lets in can open a bucket or SharePoint mount, as they
can a folder from `--mount`. The server reads it through its own connection, and AWS and Cloud
Storage know no viewer to check. A private GitHub repo is different, as "Who can open a design
linked from a private repo" below says.

`ListMounts` and `GetMount` say how each mount is reached in `kind`, where it came from in `source`,
`ref` and `commit`, and whether it is `fixed`. `ListMounts` also says in `can_create_mounts` whether
the server can make one. `make exercise-mounts` makes a GitHub mount over
Antmicro's Jetson baseboard at its 1.1.2 revision, restarts the server, and lists it again.

## Linking an artifact to a mount

A stored design's artifact can come from a mount instead of an upload (agni issue 1723). Say the
team's board lives in that GitHub repo, and they want a design on the server that reads the repo at
a release. They link the design's schematic and board to the repo at a commit:

```bash
agni designs create bb --remote http://localhost:8080
agni designs link bb schematic jetson/jetson-agx-thor-baseboard.kicad_sch@3411086e --remote http://localhost:8080
agni designs link bb board jetson/jetson-agx-thor-baseboard.kicad_pcb@3411086e --remote http://localhost:8080
```

The source is `<mount>/<path>@<ref>`. A path naming a design's entry, such as the root sheet, takes
the files a reader opens from it, the way `push` groups a folder. So the schematic above holds all
its sheets, and the board holds the board file alone. A path naming a folder takes every file under
it, which suits an ODB++ job. The server keeps its own copy of each file, by hash, so a check never
waits on GitHub. `agni designs artifacts bb` names each artifact's source and the commit it read.

A tag or a commit pins the artifact. A branch, or no ref at all, makes the artifact follow the branch,
and `agni designs sync` moves it to the branch's newest commit:

```bash
agni designs sync bb --dry-run --remote http://localhost:8080
agni designs sync bb --remote http://localhost:8080
```

A sync asks GitHub once for each mount and ref, so artifacts from one repo move to one commit
together, in one write. It downloads only the files whose git blob id it hasn't seen, so a sync of an
unchanged branch fetches nothing. Linking again at another ref moves a pinned artifact. A link to a
folder from `--mount` follows the folder, since a folder has no history, and a sync compares the
files' hashes with what it last read.

A linked artifact can't be replaced by `put` or `push` until `agni designs unlink` makes it an
uploaded one again, keeping its files. A mount with artifacts linked to it can't be deleted until
they are unlinked or deleted. The calls are `DesignService.LinkArtifact`, `UnlinkArtifact` and
`SyncDesign`, and `Artifact.source` carries the mount, path, ref, version and read time.
`make exercise-linked-sources` links Jetson 1.1.2, checks it against the same files on disk, moves
it to 1.1.3, syncs, and reads it all back after a restart. A private repo links and syncs the same
way, through the connection its mount names.

The designs page does the same from a browser (agni issue 1727). The new-design form offers "Upload
files" or "Link from a mount". Linking picks a mount, or adds one over a GitHub repo in place. Then
it walks the mount's folders to a file, takes a ref, and names the artifact by the file's kind, such
as `schematic`. The design's page links more artifacts the same way. Each linked artifact has three
controls on its row. Sync shows when the artifact follows a branch. Move reads the same path at
another ref, and Unlink keeps the files. The page also lists the design's revisions, and each opens
in the viewer as `/designs/<id>@<ref>/view`. `make exercise-linked-sources` ends by doing this in a
browser on the Jetson repo, and `web/browser/linkdesign.spec.ts` does it in the gate against a fake
GitHub.

### Reading a design at another commit

Say the team wants to see what changed between two releases of the board, without making a design
for each. A linked design reads at any ref of its source as `designs/<id>@<ref>` (agni issue 1725).
The name works wherever a design's name does:

```bash
agni diff designs/bb@3411086e designs/bb@8e087f4f --remote http://localhost:8080 --format json
agni check designs/bb@3411086e --remote http://localhost:8080
```

The server resolves the ref on each linked artifact's mount and reads the files at that commit. It
downloads only the files it doesn't hold, and leaves the design as it is. An uploaded artifact keeps
its files, and so does one linked to a `--mount` folder, which has no history. A tag or a commit
reads the same files next time with no call to GitHub. A branch is resolved again on each read.

`agni designs revisions bb` lists the revisions the server keeps, newest first. Each line names the
revision, `designs/bb@<id>`, the tags and commits that name it, and each artifact's commit. Here is
the exercise's design after it links both artifacts at 1.1.2, moves them to 1.1.3 one at a time, and
diffs the two commits:

```
designs/bb@c82028f4036a	read 2026-11-20 00:40:17; current; refs 8e087f4f946a925041380699dcde51ae8519f621; board at 8e087f4f946a925041380699dcde51ae8519f621; schematic at 8e087f4f946a925041380699dcde51ae8519f621
designs/bb@584814cd1342	read 2026-11-20 00:40:04; refs 3411086e6d6ad2384d0609c53cd6b96323f97c24; board at 3411086e6d6ad2384d0609c53cd6b96323f97c24; schematic at 3411086e6d6ad2384d0609c53cd6b96323f97c24
```

Each link writes a revision, and the server keeps the ones something names (agni issue 1774). A link
names its revision by its ref once every linked artifact is pinned at that ref, so linking both at
1.1.2 names that revision 1.1.2, and a ref names one revision at a time. A read at a ref names the
revision it gave. A report keeps the revision it ran on. When a write moves the design, the server
drops the revisions nothing names, so linking the schematic before the board, or moving them one at a
time, leaves no half-moved step behind. A sync moves every artifact from one repo together, so it
never makes one.

`agni designs revisions bb --tags` lists the linked repo's tags instead, whether the server has read
them or not. `--commits` lists a branch's commits, the branch the design follows unless `--branch`
names another. A tag the server has read, by its name or by its commit, carries the revision it gave;
one it hasn't read says "not read yet". Through a connection, tags come newest commit first. A public
repo's mount with no connection lists them too, in GitHub's version order, newest first, ten to a
page (agni issue 1788). Those requests count against GitHub's limit of 60 an hour without a token,
and a refused one is `unavailable`, saying a connection lifts it. A design whose artifacts link two
repos lists the refs of neither, since a tag names a commit in one repo.

A kept revision keeps its files on the server until it is dropped or the design is deleted. A report
names the commit each artifact was read at, such as `schematic at 8e087f4f946a`, and the revision it
read, so it still says what it ran on after a sync moves the design. `agni reports create bb@1.1.2`
checks the design as of 1.1.2, and its report sits beside the ones on the current files. The calls
are `DesignService.ListDesignRevisions`, a snapshot naming `designs/{design}@{ref}`, `CreateReport`
with that name as its parent, and `Report.source_versions` and `Report.revision`.

### Following a branch without running sync

Say someone pushes a fix to the board's `main` branch, and the design on the server follows `main`.
Nobody should have to run `agni designs sync` for the design to catch up (agni issue 1726). The
server can learn of the push in two ways. GitHub can send it a push webhook, or the server can ask
each source on a timer.

For the webhook, start the server with a secret, then ask GitHub to send the repo's pushes to it:

```bash
export AGNI_WEBHOOK_SECRET=$(openssl rand -hex 32)
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --github-webhook-secret env:AGNI_WEBHOOK_SECRET
agni mounts webhook jetson --url https://agni.example.com/hooks/github --remote http://localhost:8080
```

`--url` is the server's `/hooks/github` as GitHub reaches it, since the server can't tell its own
public address. The server makes the hook through the mount's connection, so the mount needs one,
and its token needs admin rights on the repo or a fine-grained token's webhooks permission. A token
that can only read the repo gets `permission_denied`. Running the command again updates the hook
rather than making a second one. Without a connection that may manage hooks, make the webhook in
the repo's settings by hand. Its payload URL is the same, its content type `application/json`, its
secret the server's, and its event `push`.

GitHub signs each delivery with the secret, and the server refuses one whose signature doesn't
match with 401. Every repo's webhook takes the same secret. It proves a delivery came from GitHub,
and it grants no access to any repo. A delivery for a repo moves only the artifacts linked through
that repo's mounts. Each one follows the pushed branch, by naming it or by naming no ref when the
push is to the default branch. A forged delivery could only start a sync, and the sync asks GitHub
for the branch's commit again, so the worst it does is spend API calls. The same check means a late
delivery of an older push never moves a design backward. With `--auth`, the webhook needs no token,
since the signature stands in for one.

For a server GitHub can't reach, or a link to a `--mount` folder, poll instead:

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --poll 5m
```

Each pass asks each followed branch or folder once, however many designs follow it. That is one
request to GitHub for a repo, and a walk of the file times for a folder. It syncs only the designs
whose source moved. The two flags work together, and a webhook answers in seconds where a poll waits
for its next pass.

A SharePoint or OneDrive mount can follow its drive through Microsoft Graph's change notifications
(agni issue 1729). `--msgraph-client-state` names a secret, the server takes notifications at
`/hooks/msgraph`, and `agni mounts webhook` subscribes the mount's drive there:

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --msgraph-client-state env:AGNI_GRAPH_STATE
agni mounts webhook sp --url https://agni.example.com/hooks/msgraph --remote http://localhost:8080
```

Graph checks the URL before it makes the subscription, by sending it a validation request the server
answers. So the server must already be reachable there, over https. Graph echoes the secret in each
notification, and the server checks it and drops any notification that doesn't carry it. A
notification names only the drive that changed, so the server finds every mount in that drive and
moves the designs that follow a folder there, as a push does. Graph ends a drive subscription after
about 29 days. The server keeps each one in `subscriptions/` under `--store` and renews it three
days before it ends, and `agni mounts rm` ends it. A server Graph can't reach uses `--poll`. S3 and
Cloud Storage send no events agni takes, so `--poll` follows them.

A design syncs when its source moves, unless someone set it to mark:

```bash
agni designs update bb --on-source-change mark --remote http://localhost:8080
agni designs artifacts bb --remote http://localhost:8080
```

A design that marks keeps its files, and `agni designs artifacts` ends each moved artifact's line
with `newer at the source: <commit>`. The design stays at what someone reviewed until a person runs
`agni designs sync`, which clears the mark. A link through a connection that is one person's own
can't be read when no one is viewing, so it is marked too, and the server's log says why. A
design's page in the viewer shows each artifact's source, and a Sync button beside a newer version.
A report
from before a sync says `run on earlier design files`, with the commit it ran on. The calls are
`Design.on_source_change` through `UpdateDesign`, `ArtifactSource.newer_version`, and
`WorkspaceService.CreateMountWebhook`.

### Linking a pack to a repo

Say the team keeps its rules in a repo, with a `project.yaml`, its profiles and its datasheet
parameters at the top. A pack can read them from there rather than from `agni packs push` (agni issue
1801). Mount the repo, then link the pack to a folder in it at a ref:

```bash
agni mounts add rules github://acme/agni-packs --ref main --connection acme --remote http://localhost:8080
agni packs link house rules@main --remote http://localhost:8080
agni check designs/bb --pack house --remote http://localhost:8080
```

The source reads as `designs link` reads one, `<mount>[/<path>][@<ref>]`, with the path naming the
folder that holds `project.yaml`. The server reads that folder as `packs push` reads a local one, and
keeps the files its tiers name. A branch, or no ref, is followed. The webhook and `--poll` above move
the pack to the branch's new commit, as they move a design, and `agni packs sync house` does the same
by hand. The next check runs the new rules. A tag or commit pins the pack, so a team that wants a
fixed set of rules links one.

Each report names the commit its packs were read at, as `packs/house at 3f2a9c1b04de`, beside its
design's commit. An older report says `packs/house changed since`. A pack read through a connection
opens only for a person who can read the same repo, as a design does. Linking needs read access to
the mount, so a person can't link a pack from a repo they can't read. A linked pack refuses `packs
push` and changes to its config or files. `agni packs unlink house` keeps its rules as they are and
stops following, and a pack from a private repo stays guarded after it. A mount with packs linked to
it can't be deleted. The packs page in the viewer has a form that links a pack, and each linked
pack's page has Sync and Unlink. The calls are `PackService.LinkPack`, `UnlinkPack` and `SyncPack`,
and the source is `Pack.source`.

## Connections

A private repo needs a sign-in, and a server with `--store` keeps one as a connection (agni issue
1724). Say the team's boards live in a private repo in their GitHub organisation. Either a GitHub App
installed on the organisation, or a token, lets the server read it. The server keeps a record of the
connection and never the secret: the record names where the secret is, as `env:NAME` or
`file:/abs/path`, and the server reads it from there each time it needs a token. A connection
holding a bare token is refused.

```bash
agni connections add acme --github-token env:GITHUB_TOKEN --remote http://localhost:8080
agni mounts add board github://acme/board --connection acme --remote http://localhost:8080
```

A GitHub App installation suits an organisation, since its token is scoped to the repos the
installation covers rather than to one person. The server signs a token with the App's private key,
trades it with GitHub for an installation token, and reuses that token until shortly before it
expires:

```bash
agni connections add acme-app --github-app 12345 --installation 678 --key file:/run/secrets/app.pem --remote http://localhost:8080
```

A bucket or a SharePoint folder reads through a cloud connection (agni issues 1729, 1730). Each signs
in as the server, never as a person:

```bash
agni connections add aws --aws --role-arn arn:aws:iam::123456789012:role/agni-reader --region us-east-1 --remote http://localhost:8080
agni connections add aws-keys --aws --access-key-id AKIA... --key env:AWS_SECRET_ACCESS_KEY --remote http://localhost:8080
agni connections add gcs --gcp --key file:/run/secrets/sa.json --remote http://localhost:8080
agni connections add sp --entra-tenant contoso.onmicrosoft.com --entra-client <app-id> --entra-auth secret --key env:ENTRA_SECRET --remote http://localhost:8080
```

`--aws` with no key uses the AWS default chain, such as an instance role or an IRSA role on
Kubernetes, and `--role-arn` assumes a role first. `--gcp` with no key uses Application Default
Credentials, such as GKE workload identity. An Entra connection signs in as an app registration, with
`--entra-auth` set to `secret`, `certificate` (its key at `--key`, the certificate at `--cert`),
`managed-identity` on an Azure host, or `workload-identity` on AKS, with `--key` naming the federated
token file. The app registration needs Graph's `Sites.Read.All` or `Files.Read.All` application
permission.

A server in a container, or on a machine the operator can't log in to, has no environment or files
the CLI can name. `--seal` sends the secret instead (agni issue 1802). The CLI reads the token, or
the secret `--key` names, from the reference on its own machine and sends it once. That covers an
App's key, an AWS secret key, a GCP service account key and an Entra secret or certificate key. The server seals it in
the store under `--connections-key`, as it does a person's own token, and never sends it back.
`agni connections ls` shows the connection as `sealed`. `agni connections rotate` replaces the
secret while mounts keep reading through the connection:

```bash
agni connections add acme --seal --github-token env:GITHUB_TOKEN --remote https://agni.example.com
agni connections rotate acme --github-token env:GITHUB_TOKEN_NEXT --remote https://agni.example.com
```

Several mounts can read through one connection. `agni mounts ls` names a mount's connection after
its source, and `agni connections ls` lists each connection with where its secret is and whether its
last token request worked. A connection a mount reads through can't be removed until the mount is.
The `/mounts/` page's "Add a GitHub mount" form offers the server's connections. The records live in
`connections/` under `--store`, and goapplib storage's connections package reads them. A person can
also keep a connection of their own, and on a shared server it decides which linked designs they
may open. The next section says how.

## Keeping review runs

With `--store`, the server keeps review runs in the store's `reviews/` folder. Without it the server
still serves everything else, and the review endpoints answer that they store no reviews rather than
storing a run and dropping it. The viewer's Review panel then scores the checklist without storing it
(`CreateReview` with `validate_only`) and holds the run in the page until you leave it, as it does on
the browser engine, which keeps no store at all.

The server writes the store into a directory, so in a container give it its own volume:

```
docker run -p 8080:8080 \
  -v ~/boards:/workspace/boards \
  -v agni-data:/var/lib/agni/meta \
  ghcr.io/panyam/agni:latest \
  serve --addr :8080 --mount-root /workspace \
        --store file:///var/lib/agni/meta
```

It is deliberately a different volume from your board folders.

<details>
<summary>Why the store sits outside your design mounts, and what one stored run is</summary>

Design mounts are read-only. Keeping runs somewhere else preserves that, because the server never
saves a file beside your schematics. A named volume also survives `docker rm`. Without that, storing
runs would be pointless.

Each run is one file, in the same format as `agni review --results-out`. Ordinary tools can read the
volume, and a run can be copied out and rendered anywhere. A run records
the checklist it scored, not a pointer to it, so editing your project's checklist afterwards never
rewrites what an older run says it asked.

</details>

Every client of the server can see the runs, because `agni serve` has no authentication yet. Treat
the store the way you treat the mounts. It is fine for one team on a trusted network, and it is no
boundary between teams. Nothing prunes it either, so a CI job creating a run per commit will grow the
volume until you delete runs yourself.

## The datasheet service image

The datasheets workbench has an image of its own, `ghcr.io/panyam/agnids`, published by every release
from the same tag as `agni`'s. It carries `agnids`, the workbench, and the docling environment behind
the "Extract (first pass)" action, with docling's models already inside, so Extract works on a host
with no outbound network. The docling environment makes the image about 3.7GB, where `agni`'s is
about 340MB, so the two are separate images.

```
docker run -p 8090:8090 --user $(id -u):$(id -g) -v ~/datasheets/ti:/datasheets/ti:ro -v ~/corpus:/corpus ghcr.io/panyam/agnids:latest
```

Every subdirectory of `/datasheets` becomes a mount named after itself, as `/workspace` does in the
`agni` image. They can be read-only. `/corpus` is the corpus store. It holds the workbench's drafts,
the published specs that `agni serve --params-url` reads, and under `derived/` each PDF's extracted
doc-IR and annotations, so mount a directory there to keep them. The image also reads `agni`'s
records and files from `/var/lib/agni`, so mounting the `agni` service's volume there read-only
shows the mounts `agni serve` made at runtime in the workbench too. It sees a new one within a
second, and it never writes there. A doc-IR or annotations folder an
older agnids wrote beside a PDF is still read when the store has none. To link the workbench's
heading back to a viewer, pass the whole command, because arguments after the image replace its
default rather than adding to it:

```
docker run -p 8090:8090 -v ~/datasheets/ti:/datasheets/ti ghcr.io/panyam/agnids:latest \
  serve --addr :8090 --mount-root /datasheets --web-dir /srv/agnids/web --corpus /corpus \
  --pdf2doc "/opt/docling/bin/python /opt/agnids/pdf2doc.py" --viewer-url http://host:8080
```

## Writes and file ownership

The datasheets workbench (`agnids`) writes back into what you mount. It writes an annotation or an
extracted doc-IR beside its datasheet, and a draft or a published spec into the corpus store. The
`agni` image writes only to its `--store` and `--uploads`. Both containers run as a non-root user
(uid 10001), so those files are never written as root.

On **Docker Desktop** (macOS, Windows) that is the whole story. Its file-sharing layer maps
ownership to you, so a file the container wrote as uid 10001 appears on your host owned by your own
account, and there is nothing to configure.

On **Linux**, bind mounts pass ownership through unchanged, so those files arrive owned by uid 10001
and you may not be able to edit them afterwards. Run as yourself instead:

```
docker run --user $(id -u):$(id -g) -p 8090:8090 -v ~/datasheets/ti:/datasheets/ti ghcr.io/panyam/agnids:latest
```

## What is not in the image

Two capabilities shell out to external programs and are deliberately left out of the `agni` image,
because both are large and neither is needed to read, check, diff, or query a design.

- `agni native render/open` makes native golden renders by driving the format's own tool (`kicad-cli`,
  xschem, Lepton). See [native verification](../../build/native-verification/).
- Datasheet extraction, the workbench's "Extract (first pass)" action, belongs to the separate
  datasheet service, which has [its own image](#the-datasheet-service-image).

## Running a shared server

Say a team wants one `agni serve` for everyone, with their tools and browsers on other machines. By
default the server asks nobody who they are, so anyone who can reach the port sees every mount. That
suits localhost. A shared server asks every caller for a token instead (agni issue 1627).

The operator makes a token for each person or tool on the server's machine, with the same `--store`
the server uses. `agni token create` prints the token once, and the store keeps only its hash:

```bash
agni token create alice --store file:///srv/agni/meta
```

Then the server starts with `--auth tokens`, and with a certificate so the tokens don't cross the
network in the clear:

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files \
  --auth tokens --tls-cert /etc/agni/cert.pem --tls-key /etc/agni/key.pem
```

- Every call needs `Authorization: Bearer <token>`. A call without a good one is refused as
  `unauthenticated`, in Connect, gRPC or gRPC-Web, whichever it spoke. HTTP/2 comes over TLS, so
  gRPC clients reach the server too.
- The CLI's `--remote` sends the token in `AGNI_TOKEN`. The client libraries take one when they
  connect.
- A browser that opens a page is sent to `/signin`, where its person pastes a token. The server
  keeps it in an HttpOnly session cookie. `/healthz` and the viewer's static files need no token.
- The server records who asked on what it keeps. A review run records the token's name in
  `results.meta.created_by`.
- `GetServerInfo` lists `auth` among its features. `ListMounts` leaves each mount's `root` empty,
  since a caller on another machine has no business with the server's folder layout.
- `agni token ls` lists the tokens, and `agni token revoke <id>` revokes one. The server trusts a
  token it checked for a minute, so a revoked token stops working within a minute.

`--auth tokens` needs `--store`. Without `--tls-cert` and `--tls-key` it starts only on a localhost
address, with a warning, or behind a proxy that ends TLS, as
[Behind a proxy that ends TLS](#behind-a-proxy-that-ends-tls) says. People can also sign in with GitHub, as
[Signing in with GitHub](#signing-in-with-github) says. Per-person viewer tabs are agni issue 1176.

`agnids` takes the same three flags and checks the same tokens, read from the `--store` it shares
with `agni serve` (agni issue 1765). Without them, anyone who can reach it can write through the
datasheets workbench. A browser signs in to it separately, at its own `/signin`. An `agni serve`
with `--params-url` pointing at an `agnids` that asks for tokens sends the one in its own
`AGNI_TOKEN`, so the operator makes a token for the server too.

A team whose certificate comes from its own CA hands each client the CA's file. The TypeScript client
on Node takes it as `ca`, Rust as `connect_with_ca`, Java as `AgniClient.create`'s `caFile`, and C++
as `agni::Client`'s `ca_file`. Each client's README says more. The CLI reads it from `AGNI_CA_FILE`
for every `--remote` call and for `agni login`, and trusts the system's roots beside it (agni issue
1800). On macOS that's the only way short of the Keychain, since Go there ignores `SSL_CERT_FILE`.

```bash
export AGNI_CA_FILE=/etc/agni/team-ca.pem
agni login --remote https://agni.example.com
```

### The enterprise bundle

Say a team wants that shared server on one machine, against its own GitHub Enterprise Server. Each
release attaches `agni-enterprise-<version>.tar.gz`, which unpacks into a folder holding a
`compose.yaml` and an `env.example` (agni issue 1802). The bundle runs `agni serve` with every flag
this page covers: TLS from your certificate, `--auth tokens`, sign-in through a GitHub App, a
connections key and a webhook secret. [Deploying](../deploying/) walks an admin through it, from
registering the App to checking who can open a design.

### Behind a proxy that ends TLS

Say the team runs agni on Kubernetes. The ingress holds the certificate, takes each browser's HTTPS,
and passes the request on to the pod as plain HTTP. `--auth tokens` refuses to start like that on its
own, since a token would cross the network in the clear. `--trusted-proxy` names the proxy instead
(agni issue 1810):

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files \
  --auth tokens --addr :8080 --trusted-proxy 10.0.0.0/8 \
  --public-url https://agni.example.com
```

- The server serves plain HTTP on any address. Each value is an address or a CIDR range, and the
  flag takes several, comma-separated or repeated.
- A request from a trusted proxy that says `X-Forwarded-Proto: https`, or `Forwarded: proto=https`,
  counts as HTTPS. Its session cookie is marked Secure, so the browser sends it back over HTTPS only.
- The server ignores those headers from any other peer, so a client that reaches the pod directly
  can't claim HTTPS. Its request is served as plain HTTP, and the network policy keeps such clients
  off the port.
- The proxy must set `X-Forwarded-Proto` itself rather than pass on what the client sent. Most
  ingress controllers do this already.
- Signing in with GitHub needs `--public-url`, the URL a browser reaches the proxy at. GitHub sends
  the browser back there, and the server refuses to start without it.

`agnids` takes the same flag.

### Running on Kubernetes

Each release publishes a Helm chart beside the images, as `oci://ghcr.io/panyam/charts/agni` at
the release's version (agni issue 1803). It runs the same server as the enterprise bundle, behind
your ingress controller, which ends TLS. agni trusts it through `--trusted-proxy`.
[Deploying](../deploying/#or-start-it-with-the-helm-chart) covers the Secrets, the values and the
upgrade.

### Who can open a design linked from a private repo

Say alice and bob share a server, and the server reads the team's private board repo through a
connection. Alice can see the repo on GitHub and bob can't. With `--auth tokens`, alice opens the
design and bob is refused, as GitHub would refuse him the repo (agni issue 1728).

The server decides it with each person's own GitHub connection, never with its own. Its own
connection reads the repo for everyone, so it can't tell alice from bob. A person keeps their
connection with `--mine`. The CLI reads their token from the reference on their own machine and
sends it once, and the server seals it under `--connections-key`:

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --auth tokens \
  --connections-key env:AGNI_CONNECTIONS_KEY --tls-cert /etc/agni/cert.pem --tls-key /etc/agni/key.pem
AGNI_TOKEN=... agni connections add alice-gh --mine --github-token env:GITHUB_TOKEN --remote https://agni.example
```

- `--connections-key` names where a key of 64 hex characters is, as `env:NAME` or `file:/abs/path`.
  The sealed tokens live in `usertokens/` under `--store`, and they are useless without the key.
- A design is private when an artifact of it came through a mount that reads with a connection.
  `agni designs get` then says "opens for who can read github://acme/board". The server asks GitHub
  `GET /repos/acme/board` with the caller's own token, and keeps the answer for a minute.
- A design from a public repo, a folder on the server, or an upload opens for every caller.
- Each artifact records the private repo its files came from. The record stays when the artifact
  is unlinked or the mount is deleted, so the files stay private to that repo's readers.
- Bob is refused as `permission_denied`, naming the repo, whether he asks for the design by name,
  checks it, opens a snapshot of it, or fetches one of its files. `agni designs list` leaves it out.
  When GitHub can't answer, the read is `unavailable`, and a listing names the design unreachable.
- Linking an artifact from a private mount needs the same access, so a person can't copy files
  from a repo they can't see into a design of their own.
- Only alice sees her connection. Each person's ids are their own, so bob can keep an `alice-gh`
  too, and neither learns from an error that the other has one (agni issue 1778). An id the
  server's own connections use is refused, and every caller sees those anyway. `connections/<id>`
  names the caller's own connection when they keep one by that id, else the server's.
- `agni connections rm alice-gh` deletes alice's connection and its sealed token.
  The server doesn't ask GitHub to revoke a pasted token, since alice made it there and revokes it
  there.
- A person's connection reads through the server's `--github-api`, so on GitHub Enterprise it needs
  no `--base-url`.
- `agni mounts ls` shows bob the board mount by name, with "a private source you can't read" where
  the repo would be. `ListMounts` sets `source_hidden` and leaves out its source, ref and commit.
- `agnids` with `--auth` and agni serve's `--connections-key` checks each caller's own connection
  the same way, and serves bob the mount by name with nothing under it (agni issue 1778). It reads
  the connection records and sealed tokens in `--store` and writes neither. It never refreshes a
  token either, since GitHub rotates a person's refresh token on each use and agni serve keeps the
  current one. So an expired signed-in token reads as no on agnids until agni serve refreshes it.
  A pasted token doesn't expire. Without `--connections-key`, `agnids --auth` serves no mount that
  reads through a connection.
- Connections saved before agni moved to per-person ids still read back as their owners'. The
  store moves each one to its new place on its next write.

A server without `--auth` checks nothing, since it doesn't know who is asking. It still records the
private repo on each artifact a link reads, so a design linked before `--auth` was on opens only for
the repo's readers once it is (agni issue 1778).

### Signing in with GitHub

Pasting tokens works, but each person needs a token from the operator and a GitHub token of their
own. With a GitHub App, they sign in with GitHub instead, and the server keeps the GitHub token it
gets as their own connection (agni issue 1772).

The operator registers a GitHub App first. Its callback URL is the server's
`/signin/github/callback/`, and it has "Enable Device Flow" ticked for `agni login`. The App needs
read access to Contents and Metadata on the repos people link from, and an org installs it there.
A person's token then reads only repos the App is installed on, and only those the person can see.

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --auth tokens \
  --connections-key env:AGNI_CONNECTIONS_KEY --tls-cert /etc/agni/cert.pem --tls-key /etc/agni/key.pem \
  --public-url https://agni.example.com --github-client-id Iv1.0123456789abcdef \
  --github-client-secret env:AGNI_GITHUB_CLIENT_SECRET --github-allow org:acme
agni login --remote https://agni.example.com
```

- `/signin` offers "Sign in with GitHub" beside the token form. GitHub sends the browser back to
  `--public-url`, and the server checks the person against `--github-allow`. It then sets the
  session cookie and goes on to the page they asked for.
- `--github-allow` takes `org:<org>` for an org's members and `user:<login>` for one person, comma
  separated or repeated. It can't be empty, since anyone with a GitHub account could sign in
  otherwise. Someone it leaves out gets a page saying so, and no session.
- An `org:` entry asks GitHub whether the person is in the org, through the App. That needs the
  App's Members organization permission at Read. Without it GitHub refuses to answer, so the server
  refuses each person the entry would let in, and its message names the permission (agni issue
  1841). A `user:` entry needs no permission.
- `agni login` signs in from a terminal through GitHub's device flow. It prints an address and a
  code, waits while the person types the code there, and keeps the token the server mints in
  `agni/credentials` in their config folder. Every `--remote` call to that server sends it when
  `AGNI_TOKEN` isn't set.
- A person signed in as `alice` is the caller `github:alice`. The minted token works for 30 days,
  and `agni token ls` lists it as "github sign-in". The server deletes expired tokens when it starts
  and once a day (agni issue 1778).
- Their GitHub token becomes the connection `github-alice`, owned by `github:alice`, so a design
  linked from a private repo they can read opens for them. The next sign-in replaces the token.
  goapplib refreshes it through the App, since a GitHub App's user token lasts eight hours, and
  `agni connections rm github-alice` revokes it at GitHub.
- The add-mount form on `/mounts/` and on the designs page links a person with no connection to
  sign in, and back to the page they were on. `ListMounts` names the address as `sign_in_url`.
- On localhost `--public-url` can be left out, and the server uses the address it listens on.

### Signing in through your identity provider

Say the team signs in to everything through Entra ID or Okta, and its admins won't install a GitHub
App on the org, or the boards aren't on GitHub at all. The server can sign people in through any
OpenID Connect provider instead, or beside GitHub (agni issue 1811).

The operator registers a client with the provider first. Its redirect URI is the server's
`/signin/oidc/callback/`. Turn on the device flow there too, if the provider has one, for `agni login`.

```bash
agni serve --store file:///srv/agni/meta --uploads file:///srv/agni/files --auth tokens \
  --tls-cert /etc/agni/cert.pem --tls-key /etc/agni/key.pem --public-url https://agni.example.com \
  --oidc-issuer https://login.microsoftonline.com/tenant-id/v2.0 --oidc-client-id agni-client \
  --oidc-client-secret env:AGNI_OIDC_SECRET --oidc-allow domain:example.com --oidc-label Entra
agni login --remote https://agni.example.com --with oidc
```

- The server reads the issuer's discovery document when it starts, so a wrong `--oidc-issuer` stops
  the start rather than the first sign-in. The issuer must be https, apart from a provider on
  localhost.
- `/signin` offers "Sign in with Entra", from `--oidc-label`, beside any GitHub button. The person
  signs in at the provider, and the server checks the claims of the verified ID token against
  `--oidc-allow`.
- `--oidc-allow` takes `domain:<domain>` for a verified email in a domain, `group:<id>` for a value
  in the groups claim, `sub:<sub>` for one person and `hd:<domain>` for a Google Workspace domain,
  comma separated or repeated. It can't be empty. Entra leaves the groups out of the token for
  someone in more than 200, and such a person is refused with a page saying their groups couldn't
  be checked.
- A person signed in this way is the caller `oidc:<sub>`. The minted token works for 30 days, and
  `agni token ls` lists it as "oidc sign-in" with their email. The server keeps no connection for
  them. A person who also needs private repos adds their own with `agni connections add --mine`.
- `agni login --with oidc` signs in through the provider's device flow. Without `--with`, it uses the
  server's only sign-in, and a server offering GitHub and OIDC asks which one. A provider with no
  device flow is refused by name, and the person signs in at `/signin` in a browser.
- `--oidc-client-secret` is empty for a public client. The enterprise bundle reads the `AGNI_OIDC_*`
  lines of its `.env`, and the chart its `oidc` values, and both offer GitHub alone while the issuer
  is empty.

## Analysing in the visitor's browser

The viewer's analysis can run in the visitor's browser instead of on the server. The engine is the
same Go code compiled to WebAssembly, and it answers the same calls the server does, so the panels
cannot tell which one answered. `make wasm` builds it, and then either the server or a single page
picks it:

```
agni serve --engine wasm --mount tut=examples/tutorial-project
```

A page's `?engine=wasm` or `?engine=server` overrides the server's choice for that page. On the
`wasm` engine the page fetches the design's files from the server once and analyses them in a worker,
and after that only listings and file reads reach the server. Files a visitor drops on the page stay
in the browser whichever engine is chosen.

A design whose files add up to more than `--wasm-max-bytes` (128 MB by default) goes to the server
engine instead, and the top bar says so. The default comes from measuring a 97 MB KiCad board, which
the browser read in 2.4 s and checked in 8.8 s.

With no server at all, `agni site <outdir> --base <path> --seed <name>=<folder>` writes the viewer,
the engine and the seeded designs as static files for any web host to serve.
[The public demo](../../demo/) runs this way, since the project hosts no server for it. It holds
the tutorials' gateway board and a few open-source KiCad boards from the samples corpus. You can also
drop a folder or an archive (`.zip`, `.7z`, `.tar.gz` and the like) of your own files on its "Open files" page. They stay in your browser's
storage and are never sent anywhere. An agent can't move a demo tab, since that needs a server
running `--mcp --viewer-rooms`.

## Kept reads

The server keeps what it read between requests, so a second query, check or drawing on a design
costs its own work rather than another parse of every file. On a 1,834-component KiCad board, a
repeated net count went from 656 ms to 2.5 ms and a one-rule check from 1.35 s to 18 ms.

A kept read is checked on every use against each file it opened, walked or looked for and did not
find: the entry, every sub-sheet, a declared companion, the board and the symbol libraries. Edit any
of them and the next request reads the design again. A read whose project configuration cannot be
identified is never kept. A check also keeps each rule's findings on the kept model, so a later
check of the same design runs only the rules it has not run yet. `--design-cache <n>` sets how many
reads are kept (default 12), and 0 keeps none.

## A work budget for served queries

A query that joins two large relations can run for minutes on a big board, and a shared server
answers everyone's queries from one process. `--query-budget <work>` stops any query, and any
query-backed rule a check or review runs, once it has done that much work. A query stopped this way
returns `resource_exhausted` naming the budget, and a rule stopped this way reports one inconclusive
finding saying so, since it never decided. A request can ask for a smaller budget with
`work_budget` and never for a larger one.

No budget is enforced by default, because the right number depends on the boards a deployment
serves. Instead the server logs every query costing more than `--query-budget-warn` (10,000,000
units by default) with the query and a budget ten times its cost, so a deployment can watch what
its users ask before choosing one.
This line is from the tutorial project's board with `--query-budget-warn 100`:

```
query cost: /agni.v1.webapi.QueryService/RunQuery spent 394 work units, over the 100 warning threshold; no budget is enforced, and --query-budget 3940 would allow ten times this cost: component.net(?a, ?n), component.net(?b, ?n) => ?a, ?b
```

One unit is one tuple a rule derives or one fact a lookup reads, and a circuit walk such as
`net.reaches` adds one for each connection it examines, so the number grows with the board. On a public 1,123-part sample board the heaviest shipped example query cost about one
million units. `agni query --format json` and the `RunQuery` response report what an answer cost
as `work`. A served request also stops when its caller goes away, which returns `canceled` rather
than an error about the request.

## Where a request's time goes

A slow request can say where its time went. `--slow-request <duration>` logs one line for every
request over it, naming the stage that took the most, the tier that answered each cached read, and
the slowest rules. These two lines are the tutorial project's board, asked twice with
`--slow-request 1ms`:

```
slow request: /agni.v1.webapi.CheckService/CheckDesign took 87ms, over 1ms; slowest stage resolve.overlay 44ms; cache model=built design=built board=built geometry=built; slowest rules profile/emmc-signal-missing 3ms, profile/spi_nor-signal-missing 3ms, dl/power-pin-mistyped 2ms, profile/pcie-signal-missing 2ms, profile/emmc-host-incomplete 2ms
slow request: /agni.v1.webapi.CheckService/CheckDesign took 29ms, over 1ms; slowest stage resolve.overlay 17ms; cache model=memory geometry=memory; slowest rules profile/emmc-signal-missing 2ms, profile/emmc-signal-dangling 1ms, profile/emmc-host-incomplete 1ms, gateway-profiles/can-host-incomplete 1ms, gateway-profiles/can-esd-missing 1ms
```

The second request found the model in memory, so it read nothing, and the cache column says so. If
a cache layer reads `built` on every request, the server is failing to keep that design. When
evaluating a query was the slowest stage, the line carries that query's plan as well.

Any client can ask for the same breakdown on a single request by sending an `Agni-Timing: 1` header
(`Agni-Timing: explain` adds each query's plan). The response then carries it in an `Agni-Timing`
header, as the `RequestTiming` message in protojson, and as a `Server-Timing` header. A browser's
network panel draws that header as a bar per stage. The breakdown travels in headers rather than in
the response, because a number that varied on every run would make every saved answer differ from
the last. The viewer asks for it when its address carries `?timing` (or `?timing=explain`) and logs
each request's breakdown to the console, and the browser engine answers it just as the server does.
The CLI's `--timing` on `check`, `query`, `trace` and `review` prints it from the same record, and
`query --explain` prints the plans alone.

To find which stage raised the memory peak, send `Agni-Timing: memory` instead, or open the page with
`?timing=memory`, or pass `--timing-memory` on the CLI. Each stage then carries the process's memory
as it ended, and each rule what it allocated.

Stages nest, so `build.model` includes the reads beneath it (`read.netlist`, `read.board`) and the
model it builds, and the stage list says when each started as well as how long it took.

`?timing` also times the drawing (agni issue 1331). Say a board lags when you pan it. Open the page
with `?timing`, open the board, and pan and zoom for a few seconds. Each drawing logs one line when
it opens a sheet and one more each time you stop moving it. Each line covers every pan since the
sheet opened, so the last one is enough. This one is Jetson's board on an M5 Pro, folded here one
key per line, though the console prints it on one.

```text
agni draw canvas: {
  "renderer": "webgl",
  "env": {"gpu":"ANGLE (Apple, ANGLE Metal Renderer: Apple M5 Pro, Unspecified Version)","gpuVendor":"Google Inc. (Apple)","dpr":1,"cores":18,"memoryGb":32,"cssWidth":930,"cssHeight":608},
  "scene": {"primitives":40937,"vertices":778326,"labels":5732,"images":0,"elements":0,"buildMs":56.2},
  "bursts": 1,
  "frame": {"n":19,"p50":8.3,"p95":9.3,"max":9.3},
  "draw": {"n":20,"p50":0.6,"p95":0.9,"max":1.4},
  "text": {"n":20,"p50":0,"p95":0.1,"max":0.1},
  "drawCalls": {"n":20,"p50":5069,"p95":37591,"max":39817}
}
```

`frame` is the time from one animation frame to the next one after an input, so it counts everything
the browser did to show that input. `draw` and `text` are what the WebGL drawing itself spent per
frame, in its draw calls and in moving its text layer. So when `frame` is far above `draw` plus `text`, the
browser spent the frame on layout and compositing, which on a board often means the text layer.
An SVG drawing reports `frame` and `scene.elements` alone. The line holds numbers, the renderer's
name and the GPU's name. It has no field that could carry a net name, a part or a sheet name, so a
line from a private board can go in a public issue. Send the request timings above with it, since a
slow first open shows up in those.
