from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PinColumnAxis(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PIN_COLUMN_AXIS_UNSPECIFIED: _ClassVar[PinColumnAxis]
    PIN_COLUMN_AXIS_PACKAGE: _ClassVar[PinColumnAxis]
    PIN_COLUMN_AXIS_VARIANT: _ClassVar[PinColumnAxis]
PIN_COLUMN_AXIS_UNSPECIFIED: PinColumnAxis
PIN_COLUMN_AXIS_PACKAGE: PinColumnAxis
PIN_COLUMN_AXIS_VARIANT: PinColumnAxis

class Recipe(_message.Message):
    __slots__ = ("name", "doc_title_pattern", "tables", "pin_tables", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    DOC_TITLE_PATTERN_FIELD_NUMBER: _ClassVar[int]
    TABLES_FIELD_NUMBER: _ClassVar[int]
    PIN_TABLES_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    name: str
    doc_title_pattern: str
    tables: _containers.RepeatedCompositeFieldContainer[TableRule]
    pin_tables: _containers.RepeatedCompositeFieldContainer[PinTableRule]
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., doc_title_pattern: _Optional[str] = ..., tables: _Optional[_Iterable[_Union[TableRule, _Mapping]]] = ..., pin_tables: _Optional[_Iterable[_Union[PinTableRule, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class TableRule(_message.Message):
    __slots__ = ("title_pattern", "limit_kind")
    TITLE_PATTERN_FIELD_NUMBER: _ClassVar[int]
    LIMIT_KIND_FIELD_NUMBER: _ClassVar[int]
    title_pattern: str
    limit_kind: str
    def __init__(self, title_pattern: _Optional[str] = ..., limit_kind: _Optional[str] = ...) -> None: ...

class PinTableRule(_message.Message):
    __slots__ = ("title_pattern", "column_axis")
    TITLE_PATTERN_FIELD_NUMBER: _ClassVar[int]
    COLUMN_AXIS_FIELD_NUMBER: _ClassVar[int]
    title_pattern: str
    column_axis: PinColumnAxis
    def __init__(self, title_pattern: _Optional[str] = ..., column_axis: _Optional[_Union[PinColumnAxis, str]] = ...) -> None: ...

class Patch(_message.Message):
    __slots__ = ("name", "doc_content_hash", "table_content_hash", "row", "col", "text", "note", "author")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DOC_CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    TABLE_CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    ROW_FIELD_NUMBER: _ClassVar[int]
    COL_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    name: str
    doc_content_hash: str
    table_content_hash: str
    row: int
    col: int
    text: str
    note: str
    author: str
    def __init__(self, name: _Optional[str] = ..., doc_content_hash: _Optional[str] = ..., table_content_hash: _Optional[str] = ..., row: _Optional[int] = ..., col: _Optional[int] = ..., text: _Optional[str] = ..., note: _Optional[str] = ..., author: _Optional[str] = ...) -> None: ...

class RunManifest(_message.Message):
    __slots__ = ("doc_content_hash", "doc_producer", "derive_version", "mpn", "manufacturer", "recipes", "patches_applied", "parameters_emitted", "pins_emitted", "packages_emitted", "ensemble_agreed", "ensemble_disagreed", "gaps")
    DOC_CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    DOC_PRODUCER_FIELD_NUMBER: _ClassVar[int]
    DERIVE_VERSION_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    MANUFACTURER_FIELD_NUMBER: _ClassVar[int]
    RECIPES_FIELD_NUMBER: _ClassVar[int]
    PATCHES_APPLIED_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_EMITTED_FIELD_NUMBER: _ClassVar[int]
    PINS_EMITTED_FIELD_NUMBER: _ClassVar[int]
    PACKAGES_EMITTED_FIELD_NUMBER: _ClassVar[int]
    ENSEMBLE_AGREED_FIELD_NUMBER: _ClassVar[int]
    ENSEMBLE_DISAGREED_FIELD_NUMBER: _ClassVar[int]
    GAPS_FIELD_NUMBER: _ClassVar[int]
    doc_content_hash: str
    doc_producer: str
    derive_version: str
    mpn: str
    manufacturer: str
    recipes: _containers.RepeatedScalarFieldContainer[str]
    patches_applied: _containers.RepeatedScalarFieldContainer[str]
    parameters_emitted: int
    pins_emitted: int
    packages_emitted: int
    ensemble_agreed: int
    ensemble_disagreed: int
    gaps: _containers.RepeatedCompositeFieldContainer[Gap]
    def __init__(self, doc_content_hash: _Optional[str] = ..., doc_producer: _Optional[str] = ..., derive_version: _Optional[str] = ..., mpn: _Optional[str] = ..., manufacturer: _Optional[str] = ..., recipes: _Optional[_Iterable[str]] = ..., patches_applied: _Optional[_Iterable[str]] = ..., parameters_emitted: _Optional[int] = ..., pins_emitted: _Optional[int] = ..., packages_emitted: _Optional[int] = ..., ensemble_agreed: _Optional[int] = ..., ensemble_disagreed: _Optional[int] = ..., gaps: _Optional[_Iterable[_Union[Gap, _Mapping]]] = ...) -> None: ...

class Gap(_message.Message):
    __slots__ = ("kind", "region", "detail")
    KIND_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    kind: str
    region: str
    detail: str
    def __init__(self, kind: _Optional[str] = ..., region: _Optional[str] = ..., detail: _Optional[str] = ...) -> None: ...
