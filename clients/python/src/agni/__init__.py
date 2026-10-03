"""A typed Python client for agni.

The message classes under ``agni.v1`` are generated from the same protos the engine and the web
viewer use, so a response here is the wire contract rather than a dict shaped like it.
"""

from agni.client import Client
from agni.errors import AgniError, CliUnsupported
from agni.tables import (
    diff_sheets,
    natural_key,
    natural_sort,
    review_sheet,
    rows_as_dicts,
    set_sheets,
    tables_to_xlsx,
    to_rows,
    verdict_sheets,
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
    "natural_key",
    "natural_sort",
    "parse",
    "review_sheet",
    "rows_as_dicts",
    "set_sheets",
    "tables_to_xlsx",
    "to_rows",
    "verdict_sheets",
]
