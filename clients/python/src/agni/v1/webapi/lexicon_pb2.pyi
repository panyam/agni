from agni.v1.config import naming_pb2 as _naming_pb2
from agni.v1.ir import ir_pb2 as _ir_pb2
from agni.v1.webapi import tables_pb2 as _tables_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LexiconSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LEXICON_SOURCE_UNSPECIFIED: _ClassVar[LexiconSource]
    LEXICON_SOURCE_BUILTIN: _ClassVar[LexiconSource]
    LEXICON_SOURCE_SERVER: _ClassVar[LexiconSource]
    LEXICON_SOURCE_PROJECT: _ClassVar[LexiconSource]
    LEXICON_SOURCE_DESIGN: _ClassVar[LexiconSource]
    LEXICON_SOURCE_REQUEST: _ClassVar[LexiconSource]
LEXICON_SOURCE_UNSPECIFIED: LexiconSource
LEXICON_SOURCE_BUILTIN: LexiconSource
LEXICON_SOURCE_SERVER: LexiconSource
LEXICON_SOURCE_PROJECT: LexiconSource
LEXICON_SOURCE_DESIGN: LexiconSource
LEXICON_SOURCE_REQUEST: LexiconSource

class LexiconEntry(_message.Message):
    __slots__ = ("vocabulary", "kind", "value", "source", "convention", "replaced")
    VOCABULARY_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    CONVENTION_FIELD_NUMBER: _ClassVar[int]
    REPLACED_FIELD_NUMBER: _ClassVar[int]
    vocabulary: str
    kind: str
    value: str
    source: LexiconSource
    convention: str
    replaced: bool
    def __init__(self, vocabulary: _Optional[str] = ..., kind: _Optional[str] = ..., value: _Optional[str] = ..., source: _Optional[_Union[LexiconSource, str]] = ..., convention: _Optional[str] = ..., replaced: _Optional[bool] = ...) -> None: ...

class GetLexiconRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class GetLexiconResponse(_message.Message):
    __slots__ = ("entries", "effective")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    EFFECTIVE_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[LexiconEntry]
    effective: _naming_pb2.NamingLexicon
    def __init__(self, entries: _Optional[_Iterable[_Union[LexiconEntry, _Mapping]]] = ..., effective: _Optional[_Union[_naming_pb2.NamingLexicon, _Mapping]] = ...) -> None: ...

class ExplainNetRequest(_message.Message):
    __slots__ = ("snapshot", "net")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    NET_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    net: str
    def __init__(self, snapshot: _Optional[str] = ..., net: _Optional[str] = ...) -> None: ...

class ExplainNetResponse(_message.Message):
    __slots__ = ("net", "roles", "matches")
    NET_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    MATCHES_FIELD_NUMBER: _ClassVar[int]
    net: str
    roles: _containers.RepeatedCompositeFieldContainer[_ir_pb2.NetRole]
    matches: _containers.RepeatedCompositeFieldContainer[LexiconEntry]
    def __init__(self, net: _Optional[str] = ..., roles: _Optional[_Iterable[_Union[_ir_pb2.NetRole, _Mapping]]] = ..., matches: _Optional[_Iterable[_Union[LexiconEntry, _Mapping]]] = ...) -> None: ...

class CompareLexiconsRequest(_message.Message):
    __slots__ = ("snapshot", "conventions")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    CONVENTIONS_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    conventions: _naming_pb2.NamingConvention
    def __init__(self, snapshot: _Optional[str] = ..., conventions: _Optional[_Union[_naming_pb2.NamingConvention, _Mapping]] = ...) -> None: ...

class CompareLexiconsResponse(_message.Message):
    __slots__ = ("tables",)
    TABLES_FIELD_NUMBER: _ClassVar[int]
    tables: _containers.RepeatedCompositeFieldContainer[_tables_pb2.Table]
    def __init__(self, tables: _Optional[_Iterable[_Union[_tables_pb2.Table, _Mapping]]] = ...) -> None: ...
