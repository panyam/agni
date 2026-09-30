"""Two ways to reach agni, one way to read its answer.

``ConnectTransport`` POSTs protojson to a running ``agni serve``. ``CliTransport`` runs the ``agni``
binary with ``--format json``. Both hand the text to ``parse``, because C31 makes a command's json
the protojson of its rpc's message, so there is one shape to read whichever way it arrived.
"""

from __future__ import annotations

import json
import os
import subprocess
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Callable, Dict, List, Mapping, Optional, Sequence, Type, TypeVar

from google.protobuf import json_format
from google.protobuf.message import Message

from agni.errors import AgniError, CliUnsupported
from agni.services import Rpc
from agni.v1.checks import checks_pb2 as _checks  # noqa: F401  (registers PartSpec's imports)
from agni.v1.param import param_pb2
from agni.v1.webapi import checks_pb2, design_pb2, diff_pb2, query_pb2, validate_pb2

M = TypeVar("M", bound=Message)


def parse(text: str, message_type: Type[M], *, strict: bool = False) -> M:
    """Parse protojson ``text`` into a new ``message_type``.

    With ``strict`` off (the default) a field this client does not know is skipped, so a client
    generated before a server gained a field keeps working. With it on, an unknown field is an error,
    which is what a test wants: a hand-rolled shape then fails instead of parsing into an empty
    message.
    """
    msg = message_type()
    try:
        json_format.Parse(text, msg, ignore_unknown_fields=not strict)
    except json_format.ParseError as e:
        raise AgniError(
            f"response is not a valid {message_type.DESCRIPTOR.full_name}: {e}",
            code="bad_response",
            detail=text[:2000],
        ) from e
    return msg


class ConnectTransport:
    """Calls a running ``agni serve`` over the Connect protocol's unary JSON form.

    The server reads a design once and answers many questions about it, which is the reason to
    prefer this transport when asking more than one. ``base_url`` is the server's root, such as
    ``http://127.0.0.1:8080``. Designs are named by the ``mount://`` URIs that server was started
    with.
    """

    def __init__(
        self,
        base_url: str,
        *,
        timeout: float = 300.0,
        headers: Optional[Mapping[str, str]] = None,
        strict: bool = False,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.headers = dict(headers or {})
        self.strict = strict

    def call(self, rpc: Rpc, request: Message) -> Message:
        body = json_format.MessageToJson(request).encode("utf-8")
        req = urllib.request.Request(
            self.base_url + rpc.path,
            data=body,
            method="POST",
            headers={"Content-Type": "application/json", **self.headers},
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                text = resp.read().decode("utf-8")
        except urllib.error.HTTPError as e:
            raise _connect_error(rpc, e) from None
        except urllib.error.URLError as e:
            raise AgniError(f"{rpc.path}: cannot reach {self.base_url}: {e.reason}", code="unavailable") from e
        return parse(text, rpc.response, strict=self.strict)


def _connect_error(rpc: Rpc, e: urllib.error.HTTPError) -> AgniError:
    raw = e.read().decode("utf-8", errors="replace")
    code, message = f"http_{e.code}", raw
    try:
        doc = json.loads(raw)
        code, message = doc.get("code", code), doc.get("message", raw)
    except ValueError:
        pass
    return AgniError(f"{rpc.service}/{rpc.method}: {message}", code=code, detail=raw)


# --- the CLI ---------------------------------------------------------------------------------------


@dataclass(frozen=True)
class CliCommand:
    """How one rpc is asked of the CLI.

    ``argv`` turns the request into arguments after ``agni``, and refuses a field it cannot express.
    ``emits`` is the message the command prints, and ``wrap`` lifts it into the rpc's response where
    the command prints an inner message rather than the response itself. ``answers_on_failure`` marks
    a command that prints a complete answer and then exits non-zero, which ``trace`` does for an
    endpoint naming nothing in the design. ``stdin``, when set, supplies text the command reads from
    standard input, for a request too structured for flags.
    """

    argv: Callable[[Message], List[str]]
    emits: Type[Message]
    wrap: Optional[Callable[[Message], Message]] = None
    answers_on_failure: bool = False
    stdin: Optional[Callable[[Message], str]] = None


def _only(req: Message, allowed: Sequence[str]) -> None:
    extra = [f.name for f, _ in req.ListFields() if f.name not in allowed]
    if extra:
        raise CliUnsupported(
            f"{req.DESCRIPTOR.name}: the CLI has no flag for {', '.join(sorted(extra))}; "
            "use ConnectTransport, or leave the field unset"
        )


def _read_flags(req: Message) -> List[str]:
    out: List[str] = []
    board = getattr(req, "board_uri", "")
    if board:
        out += ["--board-path", board]
    if getattr(req, "as_named", False):
        out.append("--as-named")
    return out


def _check_argv(fmt: str) -> Callable[[Message], List[str]]:
    def argv(req: Message) -> List[str]:
        _only(req, ("uri", "rules", "board_uri", "as_named"))
        out = ["check", req.uri, "--format", fmt]
        for r in req.rules:
            out += ["--rule", r]
        return out + _read_flags(req)

    return argv


def _query_argv(req: Message) -> List[str]:
    _only(req, ("uri", "query", "board_uri", "as_named"))
    return ["query", req.uri, req.query, "--format", "json"] + _read_flags(req)


def _query_set_argv(req: Message) -> List[str]:
    _only(req, ("uri", "set", "board_uri", "as_named"))
    return ["query", req.uri, "--set", "-", "--format", "json"] + _read_flags(req)


def _query_set_stdin(req: Message) -> str:
    # JSON is YAML, so the set goes to `--set -` as JSON and needs no YAML library here.
    return json.dumps(json_format.MessageToDict(req.set, preserving_proto_field_name=True))


def _diff_argv(req: Message) -> List[str]:
    _only(req, ("a_uri", "b_uri"))
    return ["diff", req.a_uri, req.b_uri, "--format", "json"]


def _trace_argv(req: Message) -> List[str]:
    _only(req, ("uri", "from", "to", "hops", "as_named"))
    src, dst = getattr(req, "from"), req.to
    out = ["trace", req.uri, "--from", f"{src.ref_des}.{src.pin}", "--to", f"{dst.ref_des}.{dst.pin}", "--format", "json"]
    if req.hops:
        out += ["--hops", str(req.hops)]
    return out + _read_flags(req)


def _layout_argv(req: Message) -> List[str]:
    _only(req, ("uri", "as_named"))
    return ["render", req.uri, "--report", "--report-format", "json"] + _read_flags(req)


# The command each rpc maps to. An rpc absent here raises CliUnsupported on the CLI transport.
#
# Deliberately absent: ReviewService, because `review --format json` is not yet the Review proto
# (agni issue 734), so mapping it would hand back a message parsed from a different shape. And
# `intake`, C31's declared exception, whose output has no wire message on purpose.
CLI_COMMANDS: Dict[str, CliCommand] = {
    "CheckService/CheckDesign": CliCommand(_check_argv("json"), checks_pb2.CheckDesignResponse),
    "CheckService/GetCheckReport": CliCommand(_check_argv("report"), checks_pb2.GetCheckReportResponse),
    "QueryService/RunQuery": CliCommand(_query_argv, query_pb2.RunQueryResponse),
    # A set with an unanswerable query prints every answer and exits 1, so the answer is still read.
    "QueryService/RunQuerySet": CliCommand(
        _query_set_argv, query_pb2.RunQuerySetResponse, stdin=_query_set_stdin, answers_on_failure=True
    ),
    "DiffService/DiffDesigns": CliCommand(_diff_argv, diff_pb2.DiffDesignsResponse),
    "DesignService/TraceDesign": CliCommand(
        _trace_argv,
        design_pb2.Trace,
        wrap=lambda t: design_pb2.TraceDesignResponse(trace=t),
        answers_on_failure=True,
    ),
    "DesignService/GetLayoutReport": CliCommand(
        _layout_argv,
        design_pb2.ConversionReport,
        wrap=lambda r: design_pb2.GetLayoutReportResponse(report=r),
    ),
}

# Commands with a wire form and no rpc behind them. Reached through CliTransport.run.
CLI_ONLY: Dict[str, Type[Message]] = {
    "validate": validate_pb2.ValidateReport,
    "params": param_pb2.PartSpec,
}


class CliTransport:
    """Runs the ``agni`` binary and parses its ``--format json``.

    Needs no server and ships as one binary, at the price of reading the design again on every call.
    Designs are named by ``mount://`` URI like the Connect transport, so one request object works on
    both; ``mounts`` maps each mount name to the folder it exposes and becomes ``--mount`` flags. A
    plain file path in a request also works here, and only here.

    ``agni`` is the binary to run. ``env`` replaces the child's environment when given, which is how a
    test keeps a personal ``~/.config/agni/agni.yaml`` out of the run.
    """

    def __init__(
        self,
        agni: str = "agni",
        *,
        mounts: Optional[Mapping[str, str]] = None,
        cwd: Optional[str] = None,
        env: Optional[Mapping[str, str]] = None,
        timeout: Optional[float] = None,
        strict: bool = False,
    ) -> None:
        self.agni = agni
        self.mounts = dict(mounts or {})
        self.cwd = cwd
        self.env = dict(env) if env is not None else None
        self.timeout = timeout
        self.strict = strict

    def call(self, rpc: Rpc, request: Message) -> Message:
        cmd = CLI_COMMANDS.get(f"{rpc.service}/{rpc.method}")
        if cmd is None:
            raise CliUnsupported(f"{rpc.service}/{rpc.method} has no CLI command; use ConnectTransport")
        stdin = cmd.stdin(request) if cmd.stdin else None
        out = self._run_text(cmd.argv(request), allow_failure=cmd.answers_on_failure, stdin=stdin)
        msg = parse(out, cmd.emits, strict=self.strict)
        return cmd.wrap(msg) if cmd.wrap else msg

    def run(self, args: Sequence[str], message_type: Type[M]) -> M:
        """Run ``agni <args>`` and parse stdout as ``message_type``.

        For a command with a wire form and no rpc, such as ``validate`` or ``params`` (see
        ``CLI_ONLY``). The caller supplies ``--format json`` along with the rest of the arguments.
        """
        return parse(self._run_text(list(args)), message_type, strict=self.strict)

    def _run_text(self, args: List[str], allow_failure: bool = False, stdin: Optional[str] = None) -> str:
        argv = [self.agni] + args
        for name, path in sorted(self.mounts.items()):
            argv += ["--mount", f"{name}={os.fspath(path)}"]
        try:
            proc = subprocess.run(
                argv,
                cwd=self.cwd,
                env=self.env,
                capture_output=True,
                text=True,
                input=stdin,
                timeout=self.timeout,
            )
        except FileNotFoundError as e:
            raise AgniError(f"cannot run {self.agni!r}: {e}", code="unavailable") from e
        if proc.returncode != 0 and not (allow_failure and proc.stdout.strip()):
            raise AgniError(
                f"agni {args[0]} exited {proc.returncode}",
                code=f"exit_{proc.returncode}",
                detail=proc.stderr.strip(),
            )
        return proc.stdout
