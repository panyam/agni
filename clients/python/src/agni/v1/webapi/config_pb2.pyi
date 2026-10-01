from agni.v1.config import naming_pb2 as _naming_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AnalysisConfig(_message.Message):
    __slots__ = ("conventions", "conventions_uri", "profile_uris", "param_uris", "checklist_uri", "intent_uri", "extends", "symbol_path_uris", "library_uris", "library_modules", "library_docs")
    class LibraryDocsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CONVENTIONS_FIELD_NUMBER: _ClassVar[int]
    CONVENTIONS_URI_FIELD_NUMBER: _ClassVar[int]
    PROFILE_URIS_FIELD_NUMBER: _ClassVar[int]
    PARAM_URIS_FIELD_NUMBER: _ClassVar[int]
    CHECKLIST_URI_FIELD_NUMBER: _ClassVar[int]
    INTENT_URI_FIELD_NUMBER: _ClassVar[int]
    EXTENDS_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_PATH_URIS_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_URIS_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_MODULES_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_DOCS_FIELD_NUMBER: _ClassVar[int]
    conventions: _naming_pb2.NamingConvention
    conventions_uri: str
    profile_uris: _containers.RepeatedScalarFieldContainer[str]
    param_uris: _containers.RepeatedScalarFieldContainer[str]
    checklist_uri: str
    intent_uri: str
    extends: str
    symbol_path_uris: _containers.RepeatedScalarFieldContainer[str]
    library_uris: _containers.RepeatedScalarFieldContainer[str]
    library_modules: _containers.RepeatedCompositeFieldContainer[LibraryModule]
    library_docs: _containers.ScalarMap[str, str]
    def __init__(self, conventions: _Optional[_Union[_naming_pb2.NamingConvention, _Mapping]] = ..., conventions_uri: _Optional[str] = ..., profile_uris: _Optional[_Iterable[str]] = ..., param_uris: _Optional[_Iterable[str]] = ..., checklist_uri: _Optional[str] = ..., intent_uri: _Optional[str] = ..., extends: _Optional[str] = ..., symbol_path_uris: _Optional[_Iterable[str]] = ..., library_uris: _Optional[_Iterable[str]] = ..., library_modules: _Optional[_Iterable[_Union[LibraryModule, _Mapping]]] = ..., library_docs: _Optional[_Mapping[str, str]] = ...) -> None: ...

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
