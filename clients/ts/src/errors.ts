// Readers for a failed call's details (agni issue 1630). A failed call throws a ConnectError. Its code
// is the gRPC code, and its details say what to do next. Each reader returns one type of detail, or
// undefined when the error has none of that type.
// docsite/content/build/client-contract.md lists the details and the codes they come with.

import { ConnectError } from "@connectrpc/connect";
import { TextPositionSchema, type TextPosition } from "./gen/agni/v1/webapi/query_pb.js";
import {
  BlobsMissingSchema,
  SnapshotUnavailableSchema,
  type BlobsMissing,
  type SnapshotUnavailable,
} from "./gen/agni/v1/webapi/snapshot_pb.js";

/**
 * textPosition says where a query or a rule stopped parsing: a byte offset, a 1-based line and
 * column, and a caret drawn under the column. It comes with invalid_argument.
 */
export function textPosition(err: unknown): TextPosition | undefined {
  return ConnectError.from(err).findDetails(TextPositionSchema)[0];
}

/**
 * snapshotUnavailable says which snapshot could not answer, and whether it is missing (not_found)
 * or stale (failed_precondition), with the tiers that changed.
 */
export function snapshotUnavailable(err: unknown): SnapshotUnavailable | undefined {
  return ConnectError.from(err).findDetails(SnapshotUnavailableSchema)[0];
}

/**
 * blobsMissing names the file contents to upload with SnapshotService.UploadBlobs before an ingest
 * of uploaded files is sent again. It comes with failed_precondition.
 */
export function blobsMissing(err: unknown): BlobsMissing | undefined {
  return ConnectError.from(err).findDetails(BlobsMissingSchema)[0];
}
