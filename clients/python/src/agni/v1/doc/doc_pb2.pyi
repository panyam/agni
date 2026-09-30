from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Document(_message.Message):
    __slots__ = ("content_hash", "source_format", "title", "producer", "page_count", "pages", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    PRODUCER_FIELD_NUMBER: _ClassVar[int]
    PAGE_COUNT_FIELD_NUMBER: _ClassVar[int]
    PAGES_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    content_hash: str
    source_format: str
    title: str
    producer: str
    page_count: int
    pages: _containers.RepeatedCompositeFieldContainer[Page]
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, content_hash: _Optional[str] = ..., source_format: _Optional[str] = ..., title: _Optional[str] = ..., producer: _Optional[str] = ..., page_count: _Optional[int] = ..., pages: _Optional[_Iterable[_Union[Page, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Page(_message.Message):
    __slots__ = ("number", "width", "height", "text_blocks", "tables", "figures")
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    TEXT_BLOCKS_FIELD_NUMBER: _ClassVar[int]
    TABLES_FIELD_NUMBER: _ClassVar[int]
    FIGURES_FIELD_NUMBER: _ClassVar[int]
    number: int
    width: float
    height: float
    text_blocks: _containers.RepeatedCompositeFieldContainer[TextBlock]
    tables: _containers.RepeatedCompositeFieldContainer[Table]
    figures: _containers.RepeatedCompositeFieldContainer[Figure]
    def __init__(self, number: _Optional[int] = ..., width: _Optional[float] = ..., height: _Optional[float] = ..., text_blocks: _Optional[_Iterable[_Union[TextBlock, _Mapping]]] = ..., tables: _Optional[_Iterable[_Union[Table, _Mapping]]] = ..., figures: _Optional[_Iterable[_Union[Figure, _Mapping]]] = ...) -> None: ...

class BBox(_message.Message):
    __slots__ = ("x", "y", "width", "height")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    x: float
    y: float
    width: float
    height: float
    def __init__(self, x: _Optional[float] = ..., y: _Optional[float] = ..., width: _Optional[float] = ..., height: _Optional[float] = ...) -> None: ...

class TextBlock(_message.Message):
    __slots__ = ("id", "text", "bbox", "kind")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    bbox: BBox
    kind: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ..., bbox: _Optional[_Union[BBox, _Mapping]] = ..., kind: _Optional[str] = ...) -> None: ...

class Table(_message.Message):
    __slots__ = ("id", "title", "section", "bbox", "rows", "cols", "cells", "footnotes", "confidence", "content_hash")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SECTION_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    COLS_FIELD_NUMBER: _ClassVar[int]
    CELLS_FIELD_NUMBER: _ClassVar[int]
    FOOTNOTES_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    section: str
    bbox: BBox
    rows: int
    cols: int
    cells: _containers.RepeatedCompositeFieldContainer[Cell]
    footnotes: _containers.RepeatedScalarFieldContainer[str]
    confidence: float
    content_hash: str
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., section: _Optional[str] = ..., bbox: _Optional[_Union[BBox, _Mapping]] = ..., rows: _Optional[int] = ..., cols: _Optional[int] = ..., cells: _Optional[_Iterable[_Union[Cell, _Mapping]]] = ..., footnotes: _Optional[_Iterable[str]] = ..., confidence: _Optional[float] = ..., content_hash: _Optional[str] = ...) -> None: ...

class Cell(_message.Message):
    __slots__ = ("row", "col", "row_span", "col_span", "text", "bbox", "is_header")
    ROW_FIELD_NUMBER: _ClassVar[int]
    COL_FIELD_NUMBER: _ClassVar[int]
    ROW_SPAN_FIELD_NUMBER: _ClassVar[int]
    COL_SPAN_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    IS_HEADER_FIELD_NUMBER: _ClassVar[int]
    row: int
    col: int
    row_span: int
    col_span: int
    text: str
    bbox: BBox
    is_header: bool
    def __init__(self, row: _Optional[int] = ..., col: _Optional[int] = ..., row_span: _Optional[int] = ..., col_span: _Optional[int] = ..., text: _Optional[str] = ..., bbox: _Optional[_Union[BBox, _Mapping]] = ..., is_header: _Optional[bool] = ...) -> None: ...

class Figure(_message.Message):
    __slots__ = ("id", "caption", "bbox", "confidence", "content_hash")
    ID_FIELD_NUMBER: _ClassVar[int]
    CAPTION_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    id: str
    caption: str
    bbox: BBox
    confidence: float
    content_hash: str
    def __init__(self, id: _Optional[str] = ..., caption: _Optional[str] = ..., bbox: _Optional[_Union[BBox, _Mapping]] = ..., confidence: _Optional[float] = ..., content_hash: _Optional[str] = ...) -> None: ...
