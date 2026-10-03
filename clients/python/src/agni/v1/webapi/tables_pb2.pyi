from agni.v1.webapi import checks_pb2 as _checks_pb2
from agni.v1.webapi import diff_pb2 as _diff_pb2
from agni.v1.webapi import query_pb2 as _query_pb2
from agni.v1.webapi import review_pb2 as _review_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ColumnType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COLUMN_TYPE_UNSPECIFIED: _ClassVar[ColumnType]
    COLUMN_TYPE_TEXT: _ClassVar[ColumnType]
    COLUMN_TYPE_NAME: _ClassVar[ColumnType]
    COLUMN_TYPE_NUMBER: _ClassVar[ColumnType]
COLUMN_TYPE_UNSPECIFIED: ColumnType
COLUMN_TYPE_TEXT: ColumnType
COLUMN_TYPE_NAME: ColumnType
COLUMN_TYPE_NUMBER: ColumnType

class TableColumn(_message.Message):
    __slots__ = ("name", "type", "kind")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    name: str
    type: ColumnType
    kind: str
    def __init__(self, name: _Optional[str] = ..., type: _Optional[_Union[ColumnType, str]] = ..., kind: _Optional[str] = ...) -> None: ...

class TableRow(_message.Message):
    __slots__ = ("cells",)
    CELLS_FIELD_NUMBER: _ClassVar[int]
    cells: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, cells: _Optional[_Iterable[str]] = ...) -> None: ...

class Table(_message.Message):
    __slots__ = ("name", "columns", "rows")
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLUMNS_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    name: str
    columns: _containers.RepeatedCompositeFieldContainer[TableColumn]
    rows: _containers.RepeatedCompositeFieldContainer[TableRow]
    def __init__(self, name: _Optional[str] = ..., columns: _Optional[_Iterable[_Union[TableColumn, _Mapping]]] = ..., rows: _Optional[_Iterable[_Union[TableRow, _Mapping]]] = ...) -> None: ...

class TabulateRequest(_message.Message):
    __slots__ = ("check", "query", "query_set", "diff", "review", "order_by", "column_types")
    class ColumnTypesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: ColumnType
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[ColumnType, str]] = ...) -> None: ...
    CHECK_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    QUERY_SET_FIELD_NUMBER: _ClassVar[int]
    DIFF_FIELD_NUMBER: _ClassVar[int]
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    ORDER_BY_FIELD_NUMBER: _ClassVar[int]
    COLUMN_TYPES_FIELD_NUMBER: _ClassVar[int]
    check: _checks_pb2.CheckDesignResponse
    query: _query_pb2.RunQueryResponse
    query_set: _query_pb2.RunQueriesResponse
    diff: _diff_pb2.DiffDesignsResponse
    review: _review_pb2.Review
    order_by: _containers.RepeatedScalarFieldContainer[str]
    column_types: _containers.ScalarMap[str, ColumnType]
    def __init__(self, check: _Optional[_Union[_checks_pb2.CheckDesignResponse, _Mapping]] = ..., query: _Optional[_Union[_query_pb2.RunQueryResponse, _Mapping]] = ..., query_set: _Optional[_Union[_query_pb2.RunQueriesResponse, _Mapping]] = ..., diff: _Optional[_Union[_diff_pb2.DiffDesignsResponse, _Mapping]] = ..., review: _Optional[_Union[_review_pb2.Review, _Mapping]] = ..., order_by: _Optional[_Iterable[str]] = ..., column_types: _Optional[_Mapping[str, ColumnType]] = ...) -> None: ...

class TabulateResponse(_message.Message):
    __slots__ = ("tables",)
    TABLES_FIELD_NUMBER: _ClassVar[int]
    tables: _containers.RepeatedCompositeFieldContainer[Table]
    def __init__(self, tables: _Optional[_Iterable[_Union[Table, _Mapping]]] = ...) -> None: ...
