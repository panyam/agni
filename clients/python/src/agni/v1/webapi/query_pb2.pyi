from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.webapi import checks_pb2 as _checks_pb2_1
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RunQueryRequest(_message.Message):
    __slots__ = ("query", "overlay", "board_uri", "uri", "as_named")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    query: str
    overlay: _checks_pb2_1.OverlayConfig
    board_uri: str
    uri: str
    as_named: bool
    def __init__(self, query: _Optional[str] = ..., overlay: _Optional[_Union[_checks_pb2_1.OverlayConfig, _Mapping]] = ..., board_uri: _Optional[str] = ..., uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class QueryRow(_message.Message):
    __slots__ = ("cells", "cites", "cell_sheets", "cell_reasons", "cell_kinds", "cell_refs")
    CELLS_FIELD_NUMBER: _ClassVar[int]
    CITES_FIELD_NUMBER: _ClassVar[int]
    CELL_SHEETS_FIELD_NUMBER: _ClassVar[int]
    CELL_REASONS_FIELD_NUMBER: _ClassVar[int]
    CELL_KINDS_FIELD_NUMBER: _ClassVar[int]
    CELL_REFS_FIELD_NUMBER: _ClassVar[int]
    cells: _containers.RepeatedScalarFieldContainer[str]
    cites: _containers.RepeatedScalarFieldContainer[str]
    cell_sheets: _containers.RepeatedCompositeFieldContainer[CellSheets]
    cell_reasons: _containers.RepeatedScalarFieldContainer[_checks_pb2.LocateReason]
    cell_kinds: _containers.RepeatedScalarFieldContainer[str]
    cell_refs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, cells: _Optional[_Iterable[str]] = ..., cites: _Optional[_Iterable[str]] = ..., cell_sheets: _Optional[_Iterable[_Union[CellSheets, _Mapping]]] = ..., cell_reasons: _Optional[_Iterable[_Union[_checks_pb2.LocateReason, str]]] = ..., cell_kinds: _Optional[_Iterable[str]] = ..., cell_refs: _Optional[_Iterable[str]] = ...) -> None: ...

class CellSheets(_message.Message):
    __slots__ = ("sheet_ids",)
    SHEET_IDS_FIELD_NUMBER: _ClassVar[int]
    sheet_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, sheet_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class RunQueryResponse(_message.Message):
    __slots__ = ("columns", "rows", "column_kinds", "query", "source")
    COLUMNS_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    COLUMN_KINDS_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    columns: _containers.RepeatedScalarFieldContainer[str]
    rows: _containers.RepeatedCompositeFieldContainer[QueryRow]
    column_kinds: _containers.RepeatedScalarFieldContainer[str]
    query: str
    source: str
    def __init__(self, columns: _Optional[_Iterable[str]] = ..., rows: _Optional[_Iterable[_Union[QueryRow, _Mapping]]] = ..., column_kinds: _Optional[_Iterable[str]] = ..., query: _Optional[str] = ..., source: _Optional[str] = ...) -> None: ...

class ListRelationsRequest(_message.Message):
    __slots__ = ("path", "uri")
    PATH_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    path: str
    uri: str
    def __init__(self, path: _Optional[str] = ..., uri: _Optional[str] = ...) -> None: ...

class RelationInfo(_message.Message):
    __slots__ = ("name", "args", "summary", "kind", "detail", "signature", "definition")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_FIELD_NUMBER: _ClassVar[int]
    name: str
    args: _containers.RepeatedScalarFieldContainer[str]
    summary: str
    kind: str
    detail: str
    signature: str
    definition: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., args: _Optional[_Iterable[str]] = ..., summary: _Optional[str] = ..., kind: _Optional[str] = ..., detail: _Optional[str] = ..., signature: _Optional[str] = ..., definition: _Optional[_Iterable[str]] = ...) -> None: ...

class RelationEntry(_message.Message):
    __slots__ = ("path", "entry_kind", "signature", "doc", "detail", "module", "definition", "inferred", "members")
    PATH_FIELD_NUMBER: _ClassVar[int]
    ENTRY_KIND_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    DOC_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    MODULE_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_FIELD_NUMBER: _ClassVar[int]
    INFERRED_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    path: str
    entry_kind: str
    signature: str
    doc: str
    detail: str
    module: str
    definition: _containers.RepeatedScalarFieldContainer[str]
    inferred: _containers.RepeatedScalarFieldContainer[str]
    members: _containers.RepeatedCompositeFieldContainer[RelationEntry]
    def __init__(self, path: _Optional[str] = ..., entry_kind: _Optional[str] = ..., signature: _Optional[str] = ..., doc: _Optional[str] = ..., detail: _Optional[str] = ..., module: _Optional[str] = ..., definition: _Optional[_Iterable[str]] = ..., inferred: _Optional[_Iterable[str]] = ..., members: _Optional[_Iterable[_Union[RelationEntry, _Mapping]]] = ...) -> None: ...

class ExampleQuery(_message.Message):
    __slots__ = ("label", "query", "teaches")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TEACHES_FIELD_NUMBER: _ClassVar[int]
    label: str
    query: str
    teaches: str
    def __init__(self, label: _Optional[str] = ..., query: _Optional[str] = ..., teaches: _Optional[str] = ...) -> None: ...

class EntityQuery(_message.Message):
    __slots__ = ("kind", "query", "teaches")
    KIND_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TEACHES_FIELD_NUMBER: _ClassVar[int]
    kind: str
    query: str
    teaches: str
    def __init__(self, kind: _Optional[str] = ..., query: _Optional[str] = ..., teaches: _Optional[str] = ...) -> None: ...

class SearchQuery(_message.Message):
    __slots__ = ("query", "teaches")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TEACHES_FIELD_NUMBER: _ClassVar[int]
    query: str
    teaches: str
    def __init__(self, query: _Optional[str] = ..., teaches: _Optional[str] = ...) -> None: ...

class ListRelationsResponse(_message.Message):
    __slots__ = ("relations", "examples", "entity_queries", "search_query", "entry")
    RELATIONS_FIELD_NUMBER: _ClassVar[int]
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    ENTITY_QUERIES_FIELD_NUMBER: _ClassVar[int]
    SEARCH_QUERY_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    relations: _containers.RepeatedCompositeFieldContainer[RelationInfo]
    examples: _containers.RepeatedCompositeFieldContainer[ExampleQuery]
    entity_queries: _containers.RepeatedCompositeFieldContainer[EntityQuery]
    search_query: SearchQuery
    entry: RelationEntry
    def __init__(self, relations: _Optional[_Iterable[_Union[RelationInfo, _Mapping]]] = ..., examples: _Optional[_Iterable[_Union[ExampleQuery, _Mapping]]] = ..., entity_queries: _Optional[_Iterable[_Union[EntityQuery, _Mapping]]] = ..., search_query: _Optional[_Union[SearchQuery, _Mapping]] = ..., entry: _Optional[_Union[RelationEntry, _Mapping]] = ...) -> None: ...

class QuerySet(_message.Message):
    __slots__ = ("title", "preamble", "queries")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    PREAMBLE_FIELD_NUMBER: _ClassVar[int]
    QUERIES_FIELD_NUMBER: _ClassVar[int]
    title: str
    preamble: str
    queries: _containers.RepeatedCompositeFieldContainer[NamedQuery]
    def __init__(self, title: _Optional[str] = ..., preamble: _Optional[str] = ..., queries: _Optional[_Iterable[_Union[NamedQuery, _Mapping]]] = ...) -> None: ...

class NamedQuery(_message.Message):
    __slots__ = ("name", "query", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    query: str
    description: str
    def __init__(self, name: _Optional[str] = ..., query: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class RunQueriesRequest(_message.Message):
    __slots__ = ("set", "uri", "overlay", "board_uri", "as_named")
    SET_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    set: QuerySet
    uri: str
    overlay: _checks_pb2_1.OverlayConfig
    board_uri: str
    as_named: bool
    def __init__(self, set: _Optional[_Union[QuerySet, _Mapping]] = ..., uri: _Optional[str] = ..., overlay: _Optional[_Union[_checks_pb2_1.OverlayConfig, _Mapping]] = ..., board_uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class RunQueriesResponse(_message.Message):
    __slots__ = ("title", "preamble", "source", "results")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    PREAMBLE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    title: str
    preamble: str
    source: str
    results: _containers.RepeatedCompositeFieldContainer[NamedQueryResult]
    def __init__(self, title: _Optional[str] = ..., preamble: _Optional[str] = ..., source: _Optional[str] = ..., results: _Optional[_Iterable[_Union[NamedQueryResult, _Mapping]]] = ...) -> None: ...

class NamedQueryResult(_message.Message):
    __slots__ = ("name", "description", "result", "error")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    result: RunQueryResponse
    error: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., result: _Optional[_Union[RunQueryResponse, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...
