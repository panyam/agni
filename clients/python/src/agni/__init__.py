"""A typed Python client for agni.

The message classes under ``agni.v1`` are generated from the same protos the engine and the web
viewer use, so a response here is the wire contract rather than a dict shaped like it.
"""

from agni.client import Client
from agni.errors import AgniError, CliUnsupported
from agni.tables import (
    diff_sheets,
    review_sheet,
    rows_as_dicts,
    table_sheets,
    tables_to_xlsx,
    to_rows,
)
from agni.transport import CLI_COMMANDS, CLI_ONLY, CliTransport, ConnectTransport, parse
from agni.values import bindings

__all__ = [
    "AgniError",
    "bindings",
    "CLI_COMMANDS",
    "CLI_ONLY",
    "Client",
    "CliTransport",
    "CliUnsupported",
    "ConnectTransport",
    "diff_sheets",
    "parse",
    "review_sheet",
    "rows_as_dicts",
    "table_sheets",
    "tables_to_xlsx",
    "to_rows",
]
