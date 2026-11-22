# follow-a-bucket

Mount an S3 prefix on a server, see that the mount keeps the files it read, and how a SharePoint
folder mounts and tells the server when it changes (agni issues 1729, 1730). The walkthrough form of
`agni connections add --aws`, `agni mounts add <name> s3://...` and `agni mounts webhook` on a
SharePoint mount.

## What it shows

- An AWS connection signs S3 requests, and `CreateMount` snapshots `s3://<bucket>/<prefix>`.
- The mount's root is a folder on the server, since the engine reads a design through a folder.
- A snapshot pins each object at its version, so an overwritten object leaves the mount as it was.
- A SharePoint folder mounts through an Entra connection, and Graph's notifications move the designs
  that follow it.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md). [`tour.go`](tour.go) builds a mount
table the way `agni serve --store --uploads` does, over gofakes3, an S3 in the process.
[`main.go`](main.go) binds the two steps that run code. [`tour_test.go`](tour_test.go) runs the same
calls and checks their answers. `cmd/agni/bucketmounts_test.go` covers SharePoint end to end, with
goapplib's fake Graph and Entra.
