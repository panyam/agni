package io.github.panyam.agni.client;

import com.google.protobuf.Any;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Message;
import io.github.panyam.agni.v1.webapi.BlobsMissing;
import io.github.panyam.agni.v1.webapi.SnapshotUnavailable;
import io.github.panyam.agni.v1.webapi.TextPosition;
import io.grpc.Status;
import io.grpc.protobuf.StatusProto;
import java.util.List;
import java.util.Locale;
import java.util.Optional;

/**
 * Reads what a failed call carries. A failed rpc has a code and a list of details, the same over
 * Connect and gRPC, and the details say what to do next:
 *
 * <ul>
 *   <li>{@link TextPosition} comes with {@code invalid_argument} from a query or a rule. It says
 *       where in the text parsing stopped.
 *   <li>{@link SnapshotUnavailable} comes with {@code not_found} or {@code failed_precondition}. It
 *       names the snapshot and says whether it is missing or stale.
 *   <li>{@link BlobsMissing} comes with {@code failed_precondition} from an ingest of uploaded
 *       files. It lists the contents to upload before ingesting again.
 * </ul>
 *
 * <p>Each method takes the {@link Throwable} a stub threw, usually a {@link
 * io.grpc.StatusRuntimeException}, and answers empty for anything that carries no gRPC status.
 */
public final class AgniErrors {
  private AgniErrors() {}

  /**
   * The call's code as Connect and the wire fixtures name it, such as {@code not_found}, or empty
   * when the throwable carries no gRPC status. gRPC spells one code differently: its CANCELLED is
   * Connect's {@code canceled}.
   */
  public static Optional<String> code(Throwable t) {
    Status s = statusOf(t);
    if (s == null) {
      return Optional.empty();
    }
    if (s.getCode() == Status.Code.CANCELLED) {
      return Optional.of("canceled");
    }
    return Optional.of(s.getCode().name().toLowerCase(Locale.ROOT));
  }

  /** Every detail the call carries, packed, in the server's order. */
  public static List<Any> details(Throwable t) {
    com.google.rpc.Status s = StatusProto.fromThrowable(t);
    return s == null ? List.of() : s.getDetailsList();
  }

  /** The first detail of type {@code type}, unpacked. */
  public static <T extends Message> Optional<T> detail(Throwable t, Class<T> type) {
    for (Any d : details(t)) {
      if (d.is(type)) {
        try {
          return Optional.of(d.unpack(type));
        } catch (InvalidProtocolBufferException e) {
          throw new IllegalStateException("agni: the server sent a detail that does not decode", e);
        }
      }
    }
    return Optional.empty();
  }

  /** Where a query or rule stopped parsing, from an {@code invalid_argument}. */
  public static Optional<TextPosition> textPosition(Throwable t) {
    return detail(t, TextPosition.class);
  }

  /** Which snapshot cannot answer, and why, from a {@code not_found} or {@code failed_precondition}. */
  public static Optional<SnapshotUnavailable> snapshotUnavailable(Throwable t) {
    return detail(t, SnapshotUnavailable.class);
  }

  /** The file contents to upload before ingesting again, from a {@code failed_precondition}. */
  public static Optional<BlobsMissing> blobsMissing(Throwable t) {
    return detail(t, BlobsMissing.class);
  }

  private static Status statusOf(Throwable t) {
    for (Throwable c = t; c != null; c = c.getCause()) {
      if (c instanceof io.grpc.StatusRuntimeException e) {
        return e.getStatus();
      }
      if (c instanceof io.grpc.StatusException e) {
        return e.getStatus();
      }
    }
    return null;
  }
}
