"""Errors raised by both transports."""

from __future__ import annotations

from typing import Any


class AgniError(Exception):
    """A call agni answered with an error, or could not answer at all.

    ``code`` is the Connect error code (``"invalid_argument"``, ``"not_found"``, ...) over HTTP, and
    ``"exit_<n>"`` from the CLI, where the process has no finer vocabulary. ``detail`` carries what
    the other side said verbatim: the Connect error body, or the CLI's stderr.

    ``snapshot_unavailable`` is the one detail parsed out of a Connect error, a
    ``SnapshotUnavailable`` naming a snapshot the server no longer holds or whose files changed,
    because ``Client`` recovers from it (agni issue 1067). It is None on every other error.
    """

    def __init__(self, message: str, *, code: str = "unknown", detail: str = "", snapshot_unavailable: Any = None) -> None:
        super().__init__(message)
        self.code = code
        self.detail = detail
        self.snapshot_unavailable = snapshot_unavailable

    def __str__(self) -> str:
        base = super().__str__()
        return f"{base}\n{self.detail}" if self.detail else base


class CliUnsupported(AgniError):
    """The CLI transport cannot express this call.

    Raised for an rpc no command maps to, and for a request field the mapped command has no flag
    for. Dropping a field silently would answer a different question from the one asked, which is
    worse than refusing, so the CLI transport refuses.
    """

    def __init__(self, message: str) -> None:
        super().__init__(message, code="unsupported_on_cli")
