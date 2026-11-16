package io.github.panyam.agni.client;

import io.github.panyam.agni.v1.webapi.ServerInfo;

/**
 * Thrown by {@link AgniClient#connect()} when the server speaks a proto package other than {@link
 * AgniClient#API}. No rpc in this client can rely on such a server's shapes.
 */
public final class IncompatibleServerException extends RuntimeException {
  private final transient ServerInfo info;

  IncompatibleServerException(ServerInfo info) {
    super(
        "agni: the server speaks \""
            + info.getApi()
            + "\", and this client was built for "
            + AgniClient.API
            + " (server version "
            + info.getVersion()
            + ")");
    this.info = info;
  }

  /** What the server said about itself. */
  public ServerInfo serverInfo() {
    return info;
  }
}
