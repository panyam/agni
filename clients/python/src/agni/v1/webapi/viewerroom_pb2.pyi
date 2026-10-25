import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class JoinViewerRoomRequest(_message.Message):
    __slots__ = ("design",)
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    design: str
    def __init__(self, design: _Optional[str] = ...) -> None: ...

class JoinViewerRoomResponse(_message.Message):
    __slots__ = ("room", "relay_url", "token", "expire_time", "bridge_subjects")
    ROOM_FIELD_NUMBER: _ClassVar[int]
    RELAY_URL_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    BRIDGE_SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    room: str
    relay_url: str
    token: str
    expire_time: _timestamp_pb2.Timestamp
    bridge_subjects: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, room: _Optional[str] = ..., relay_url: _Optional[str] = ..., token: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., bridge_subjects: _Optional[_Iterable[str]] = ...) -> None: ...
