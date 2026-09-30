from agni.v1.param import param_pb2 as _param_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Request(_message.Message):
    __slots__ = ("mpn", "symbol")
    MPN_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    mpn: str
    symbol: str
    def __init__(self, mpn: _Optional[str] = ..., symbol: _Optional[str] = ...) -> None: ...

class Citation(_message.Message):
    __slots__ = ("page", "region_id", "row", "col", "quote")
    PAGE_FIELD_NUMBER: _ClassVar[int]
    REGION_ID_FIELD_NUMBER: _ClassVar[int]
    ROW_FIELD_NUMBER: _ClassVar[int]
    COL_FIELD_NUMBER: _ClassVar[int]
    QUOTE_FIELD_NUMBER: _ClassVar[int]
    page: int
    region_id: str
    row: int
    col: int
    quote: str
    def __init__(self, page: _Optional[int] = ..., region_id: _Optional[str] = ..., row: _Optional[int] = ..., col: _Optional[int] = ..., quote: _Optional[str] = ...) -> None: ...

class Candidate(_message.Message):
    __slots__ = ("request", "citation", "value", "unit", "limit_kind", "source", "confidence")
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    CITATION_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    LIMIT_KIND_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    request: Request
    citation: Citation
    value: _param_pb2.RangeValue
    unit: str
    limit_kind: _param_pb2.LimitKind
    source: str
    confidence: float
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ..., citation: _Optional[_Union[Citation, _Mapping]] = ..., value: _Optional[_Union[_param_pb2.RangeValue, _Mapping]] = ..., unit: _Optional[str] = ..., limit_kind: _Optional[_Union[_param_pb2.LimitKind, str]] = ..., source: _Optional[str] = ..., confidence: _Optional[float] = ...) -> None: ...
