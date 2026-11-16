from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class GetServerInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ServerInfo(_message.Message):
    __slots__ = ("version", "api", "features")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    API_FIELD_NUMBER: _ClassVar[int]
    FEATURES_FIELD_NUMBER: _ClassVar[int]
    version: str
    api: str
    features: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, version: _Optional[str] = ..., api: _Optional[str] = ..., features: _Optional[_Iterable[str]] = ...) -> None: ...
