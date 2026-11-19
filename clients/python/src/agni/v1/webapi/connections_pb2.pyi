import datetime

from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ConnectionKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONNECTION_KIND_UNSPECIFIED: _ClassVar[ConnectionKind]
    CONNECTION_KIND_GITHUB_APP: _ClassVar[ConnectionKind]
    CONNECTION_KIND_GITHUB_TOKEN: _ClassVar[ConnectionKind]

class ConnectionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONNECTION_STATUS_UNSPECIFIED: _ClassVar[ConnectionStatus]
    CONNECTION_STATUS_OK: _ClassVar[ConnectionStatus]
    CONNECTION_STATUS_FAILED: _ClassVar[ConnectionStatus]
CONNECTION_KIND_UNSPECIFIED: ConnectionKind
CONNECTION_KIND_GITHUB_APP: ConnectionKind
CONNECTION_KIND_GITHUB_TOKEN: ConnectionKind
CONNECTION_STATUS_UNSPECIFIED: ConnectionStatus
CONNECTION_STATUS_OK: ConnectionStatus
CONNECTION_STATUS_FAILED: ConnectionStatus

class Connection(_message.Message):
    __slots__ = ("name", "kind", "display_name", "base_url", "app_id", "installation_id", "key_source", "token_source", "status", "last_error", "create_time", "last_use_time")
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    APP_ID_FIELD_NUMBER: _ClassVar[int]
    INSTALLATION_ID_FIELD_NUMBER: _ClassVar[int]
    KEY_SOURCE_FIELD_NUMBER: _ClassVar[int]
    TOKEN_SOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    LAST_ERROR_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_USE_TIME_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: ConnectionKind
    display_name: str
    base_url: str
    app_id: str
    installation_id: int
    key_source: str
    token_source: str
    status: ConnectionStatus
    last_error: str
    create_time: _timestamp_pb2.Timestamp
    last_use_time: _timestamp_pb2.Timestamp
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[_Union[ConnectionKind, str]] = ..., display_name: _Optional[str] = ..., base_url: _Optional[str] = ..., app_id: _Optional[str] = ..., installation_id: _Optional[int] = ..., key_source: _Optional[str] = ..., token_source: _Optional[str] = ..., status: _Optional[_Union[ConnectionStatus, str]] = ..., last_error: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_use_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateConnectionRequest(_message.Message):
    __slots__ = ("connection_id", "connection")
    CONNECTION_ID_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    connection_id: str
    connection: Connection
    def __init__(self, connection_id: _Optional[str] = ..., connection: _Optional[_Union[Connection, _Mapping]] = ...) -> None: ...

class GetConnectionRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListConnectionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListConnectionsResponse(_message.Message):
    __slots__ = ("connections",)
    CONNECTIONS_FIELD_NUMBER: _ClassVar[int]
    connections: _containers.RepeatedCompositeFieldContainer[Connection]
    def __init__(self, connections: _Optional[_Iterable[_Union[Connection, _Mapping]]] = ...) -> None: ...

class DeleteConnectionRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...
