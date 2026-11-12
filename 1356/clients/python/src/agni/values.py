"""Values a query's variables are bound to."""

from __future__ import annotations

from typing import Dict, Mapping, Sequence, Union

from agni.v1.webapi import query_pb2


Scalar = Union[str, int, float]


def bindings(values: Mapping[str, Union[Scalar, Sequence[Scalar]]]) -> Dict[str, query_pb2.QueryValue]:
    """Turn plain values into a request's ``bindings`` (agni issue 793)::

        c.run_query(uri=..., query="component.net(?r, ?n) => ?n", bindings=agni.bindings({"r": "U1"}))

    A ``str`` binds text and an ``int`` or ``float`` a number. Where the query's relations type the
    variable, the engine reads the value as that type either way, so ``"3"`` against a count is 3
    and ``"abc"`` is refused. Where nothing types it, the Python type decides, as ``"3"`` versus
    ``3`` written into the query would. A ``list`` or ``tuple`` of those binds the variable to each
    value, and the query answers for every one (agni issue 1121). Naming a variable the query does
    not use is an error.
    """
    out: Dict[str, query_pb2.QueryValue] = {}
    for name, v in values.items():
        if isinstance(v, (list, tuple)):
            out[name] = query_pb2.QueryValue(list=query_pb2.QueryValueList(values=[_scalar(name, e) for e in v]))
        else:
            out[name] = _scalar(name, v)
    return out


def _scalar(name: str, v: object) -> query_pb2.QueryValue:
    if isinstance(v, bool) or not isinstance(v, (str, int, float)):
        raise TypeError(f"binding {name}: want str, int, float or a list of them, got {type(v).__name__}")
    return query_pb2.QueryValue(text=v) if isinstance(v, str) else query_pb2.QueryValue(number=float(v))
