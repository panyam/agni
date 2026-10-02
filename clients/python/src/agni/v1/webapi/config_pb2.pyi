from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.config import intent_pb2 as _intent_pb2
from agni.v1.config import naming_pb2 as _naming_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AnalysisConfig(_message.Message):
    __slots__ = ("conventions", "profile_uris", "param_uris", "checklists", "intent", "extends", "symbol_path_uris", "library_uris", "library_modules", "library_docs")
    class LibraryDocsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CONVENTIONS_FIELD_NUMBER: _ClassVar[int]
    PROFILE_URIS_FIELD_NUMBER: _ClassVar[int]
    PARAM_URIS_FIELD_NUMBER: _ClassVar[int]
    CHECKLISTS_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    EXTENDS_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_PATH_URIS_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_URIS_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_MODULES_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_DOCS_FIELD_NUMBER: _ClassVar[int]
    conventions: _naming_pb2.NamingConvention
    profile_uris: _containers.RepeatedScalarFieldContainer[str]
    param_uris: _containers.RepeatedScalarFieldContainer[str]
    checklists: _containers.RepeatedCompositeFieldContainer[NamedChecklist]
    intent: _intent_pb2.DesignIntent
    extends: str
    symbol_path_uris: _containers.RepeatedScalarFieldContainer[str]
    library_uris: _containers.RepeatedScalarFieldContainer[str]
    library_modules: _containers.RepeatedCompositeFieldContainer[LibraryModule]
    library_docs: _containers.ScalarMap[str, str]
    def __init__(self, conventions: _Optional[_Union[_naming_pb2.NamingConvention, _Mapping]] = ..., profile_uris: _Optional[_Iterable[str]] = ..., param_uris: _Optional[_Iterable[str]] = ..., checklists: _Optional[_Iterable[_Union[NamedChecklist, _Mapping]]] = ..., intent: _Optional[_Union[_intent_pb2.DesignIntent, _Mapping]] = ..., extends: _Optional[str] = ..., symbol_path_uris: _Optional[_Iterable[str]] = ..., library_uris: _Optional[_Iterable[str]] = ..., library_modules: _Optional[_Iterable[_Union[LibraryModule, _Mapping]]] = ..., library_docs: _Optional[_Mapping[str, str]] = ...) -> None: ...

class NamedChecklist(_message.Message):
    __slots__ = ("name", "manifest")
    NAME_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    name: str
    manifest: _checks_pb2.ReviewManifest
    def __init__(self, name: _Optional[str] = ..., manifest: _Optional[_Union[_checks_pb2.ReviewManifest, _Mapping]] = ...) -> None: ...

class LibraryModule(_message.Message):
    __slots__ = ("path", "language", "text", "source")
    PATH_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    path: str
    language: str
    text: str
    source: str
    def __init__(self, path: _Optional[str] = ..., language: _Optional[str] = ..., text: _Optional[str] = ..., source: _Optional[str] = ...) -> None: ...
