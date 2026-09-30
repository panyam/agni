from agni.v1.config import naming_pb2 as _naming_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AnalysisConfig(_message.Message):
    __slots__ = ("conventions", "conventions_uri", "profile_uris", "param_uris", "checklist_uri", "intent_uri", "extends", "symbol_path_uris")
    CONVENTIONS_FIELD_NUMBER: _ClassVar[int]
    CONVENTIONS_URI_FIELD_NUMBER: _ClassVar[int]
    PROFILE_URIS_FIELD_NUMBER: _ClassVar[int]
    PARAM_URIS_FIELD_NUMBER: _ClassVar[int]
    CHECKLIST_URI_FIELD_NUMBER: _ClassVar[int]
    INTENT_URI_FIELD_NUMBER: _ClassVar[int]
    EXTENDS_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_PATH_URIS_FIELD_NUMBER: _ClassVar[int]
    conventions: _naming_pb2.NamingConvention
    conventions_uri: str
    profile_uris: _containers.RepeatedScalarFieldContainer[str]
    param_uris: _containers.RepeatedScalarFieldContainer[str]
    checklist_uri: str
    intent_uri: str
    extends: str
    symbol_path_uris: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, conventions: _Optional[_Union[_naming_pb2.NamingConvention, _Mapping]] = ..., conventions_uri: _Optional[str] = ..., profile_uris: _Optional[_Iterable[str]] = ..., param_uris: _Optional[_Iterable[str]] = ..., checklist_uri: _Optional[str] = ..., intent_uri: _Optional[str] = ..., extends: _Optional[str] = ..., symbol_path_uris: _Optional[_Iterable[str]] = ...) -> None: ...
