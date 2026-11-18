---
title: Keep a design on a server
description: Store two revisions of a board as designs of one series, group them in a project, and see what a mounted folder will not let you change
actors:
  - id: You
    label: You
  - id: Agni
    label: agni server
---

## A design is its artifacts

Picture an engineer with a board and no folder on the server. They want the server to keep the board under a name, across a restart, and to keep the next revision beside it. A server started with `--store` and `--uploads` does that (agni issues 1694, 1739).

A design is named `designs/{design}` and holds artifacts (agni issue 1696). An artifact is one source of the design, such as a netlist, a schematic with its sheets, or a board, and replacing it replaces all its files. The server reads each artifact to find its type. It keeps the design's record in its store, and each file once, by hash, in its uploads. A design ties to no project. A project is a title and a list of design names, so one design can sit in several projects. This walkthrough builds the same two services `agni serve --store --uploads` builds, over a temporary folder, with the bundled demo project mounted read-only beside it.

## Keep the first revision {#keep}

> The client makes the design with its title and series, then puts its netlist as an artifact. It sends the artifact's manifest first: each file's path, SHA-256 and size. The server answers with the contents it lacks, the client sends those with `UploadBlobs`, and asks again. A file the server already holds is never sent twice.

```mermaid
sequenceDiagram
You ->> Agni: CreateDesign(mixer-rev-a, series mixer)
You ->> Agni: UpdateArtifact(netlist, manifest, allow_missing)
Agni -->> You: FAILED_PRECONDITION, BlobsMissing [1 hash]
You ->> Agni: UploadBlobs(contents)
You ->> Agni: UpdateArtifact(netlist, manifest, allow_missing)
Agni -->> You: designs/mixer-rev-a/artifacts/netlist
```

## Keep the next revision in the same series {#again}

> Each revision is a design of its own. The series `mixer` groups them, and the server marks the one it updated last.

```mermaid
sequenceDiagram
You ->> Agni: CreateDesign(mixer-rev-b, series mixer)
You ->> Agni: UpdateArtifact(netlist, manifest, allow_missing)
You ->> Agni: ListDesigns(series = "mixer")
Agni -->> You: mixer-rev-a, mixer-rev-b (latest)
```

## Keep a pack of rules {#pack}

> A pack holds the rules a design is judged by, in the shape a `project.yaml` holds them: a naming convention, checklists, profiles, params and intent (agni issue 1704). It is named `packs/{pack}` and ties to no design. A snapshot names the packs it is read under, and `agni check designs/<d> --pack house` checks a stored design against one. The server hashes each pack, so a report can later say the pack changed since it ran.

```mermaid
sequenceDiagram
You ->> Agni: CreatePack(house, convention)
Agni -->> You: packs/house, hash A
You ->> Agni: UpdatePack(house, new rail pattern)
Agni -->> You: packs/house, hash B
```

## Group them in a project {#group}

> The project lists the two stored revisions and the mounted `designs/mixer` together. A design's name says nothing about where its files live, so one project can hold designs from any store.

```mermaid
sequenceDiagram
You ->> Agni: CreateProject(lab, [mixer, mixer-rev-a, mixer-rev-b])
You ->> Agni: ListDesigns(project = "projects/lab")
Agni -->> You: three designs
```

## A mounted folder is read-only {#mounted}

> The demo project's folder belongs to whoever mounted it. The server reads it and refuses to change it, with FAILED_PRECONDITION rather than NOT_FOUND, so the caller knows the design exists.

## Delete the project, keep the designs {#ungroup}

> A project only lists designs, so deleting it deletes none of them.

```mermaid
sequenceDiagram
You ->> Agni: DeleteProject(projects/lab)
You ->> Agni: ListDesigns()
Agni -->> You: three designs
```
