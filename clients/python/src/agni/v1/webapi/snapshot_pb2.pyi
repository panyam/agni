from agni.v1.webapi import config_pb2 as _config_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Snapshot(_message.Message):
    __slots__ = ("name", "revision_id", "uri", "board_uri", "geometry_uri", "as_named", "overlay", "tiers", "consistency", "project", "files", "flatten_hierarchy")
    NAME_FIELD_NUMBER: _ClassVar[int]
    REVISION_ID_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    GEOMETRY_URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    TIERS_FIELD_NUMBER: _ClassVar[int]
    CONSISTENCY_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    FLATTEN_HIERARCHY_FIELD_NUMBER: _ClassVar[int]
    name: str
    revision_id: str
    uri: str
    board_uri: str
    geometry_uri: str
    as_named: bool
    overlay: _config_pb2.OverlayConfig
    tiers: SnapshotTiers
    consistency: SnapshotConsistency
    project: str
    files: _containers.RepeatedCompositeFieldContainer[SnapshotFile]
    flatten_hierarchy: bool
    def __init__(self, name: _Optional[str] = ..., revision_id: _Optional[str] = ..., uri: _Optional[str] = ..., board_uri: _Optional[str] = ..., geometry_uri: _Optional[str] = ..., as_named: _Optional[bool] = ..., overlay: _Optional[_Union[_config_pb2.OverlayConfig, _Mapping]] = ..., tiers: _Optional[_Union[SnapshotTiers, _Mapping]] = ..., consistency: _Optional[_Union[SnapshotConsistency, _Mapping]] = ..., project: _Optional[str] = ..., files: _Optional[_Iterable[_Union[SnapshotFile, _Mapping]]] = ..., flatten_hierarchy: _Optional[bool] = ...) -> None: ...

class SnapshotFile(_message.Message):
    __slots__ = ("path", "sha256", "size")
    PATH_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    path: str
    sha256: str
    size: int
    def __init__(self, path: _Optional[str] = ..., sha256: _Optional[str] = ..., size: _Optional[int] = ...) -> None: ...

class SnapshotTiers(_message.Message):
    __slots__ = ("netlist_uri", "board_uri", "geometry_uri", "board_read", "board_persisted")
    NETLIST_URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    GEOMETRY_URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_READ_FIELD_NUMBER: _ClassVar[int]
    BOARD_PERSISTED_FIELD_NUMBER: _ClassVar[int]
    netlist_uri: str
    board_uri: str
    geometry_uri: str
    board_read: bool
    board_persisted: bool
    def __init__(self, netlist_uri: _Optional[str] = ..., board_uri: _Optional[str] = ..., geometry_uri: _Optional[str] = ..., board_read: _Optional[bool] = ..., board_persisted: _Optional[bool] = ...) -> None: ...

class SnapshotConsistency(_message.Message):
    __slots__ = ("board_only", "board_only_count", "netlist_only", "netlist_only_count")
    BOARD_ONLY_FIELD_NUMBER: _ClassVar[int]
    BOARD_ONLY_COUNT_FIELD_NUMBER: _ClassVar[int]
    NETLIST_ONLY_FIELD_NUMBER: _ClassVar[int]
    NETLIST_ONLY_COUNT_FIELD_NUMBER: _ClassVar[int]
    board_only: _containers.RepeatedScalarFieldContainer[str]
    board_only_count: int
    netlist_only: _containers.RepeatedScalarFieldContainer[str]
    netlist_only_count: int
    def __init__(self, board_only: _Optional[_Iterable[str]] = ..., board_only_count: _Optional[int] = ..., netlist_only: _Optional[_Iterable[str]] = ..., netlist_only_count: _Optional[int] = ...) -> None: ...

class IngestRequest(_message.Message):
    __slots__ = ("snapshot", "snapshot_id", "read_board")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    SNAPSHOT_ID_FIELD_NUMBER: _ClassVar[int]
    READ_BOARD_FIELD_NUMBER: _ClassVar[int]
    snapshot: Snapshot
    snapshot_id: str
    read_board: bool
    def __init__(self, snapshot: _Optional[_Union[Snapshot, _Mapping]] = ..., snapshot_id: _Optional[str] = ..., read_board: _Optional[bool] = ...) -> None: ...

class GetSnapshotRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListSnapshotsRequest(_message.Message):
    __slots__ = ("page_size", "page_token", "filter")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    filter: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ...) -> None: ...

class ListSnapshotsResponse(_message.Message):
    __slots__ = ("snapshots", "next_page_token")
    SNAPSHOTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    snapshots: _containers.RepeatedCompositeFieldContainer[Snapshot]
    next_page_token: str
    def __init__(self, snapshots: _Optional[_Iterable[_Union[Snapshot, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class UpdateSnapshotRequest(_message.Message):
    __slots__ = ("snapshot", "update_mask", "read_board")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    READ_BOARD_FIELD_NUMBER: _ClassVar[int]
    snapshot: Snapshot
    update_mask: _field_mask_pb2.FieldMask
    read_board: bool
    def __init__(self, snapshot: _Optional[_Union[Snapshot, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ..., read_board: _Optional[bool] = ...) -> None: ...

class DeleteSnapshotRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class BlobsMissing(_message.Message):
    __slots__ = ("sha256",)
    SHA256_FIELD_NUMBER: _ClassVar[int]
    sha256: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, sha256: _Optional[_Iterable[str]] = ...) -> None: ...

class UploadBlobsRequest(_message.Message):
    __slots__ = ("contents",)
    CONTENTS_FIELD_NUMBER: _ClassVar[int]
    contents: _containers.RepeatedScalarFieldContainer[bytes]
    def __init__(self, contents: _Optional[_Iterable[bytes]] = ...) -> None: ...

class UploadBlobsResponse(_message.Message):
    __slots__ = ("sha256",)
    SHA256_FIELD_NUMBER: _ClassVar[int]
    sha256: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, sha256: _Optional[_Iterable[str]] = ...) -> None: ...

class SnapshotUnavailable(_message.Message):
    __slots__ = ("name", "reason", "changed")
    class Reason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        REASON_UNSPECIFIED: _ClassVar[SnapshotUnavailable.Reason]
        REASON_MISSING: _ClassVar[SnapshotUnavailable.Reason]
        REASON_STALE: _ClassVar[SnapshotUnavailable.Reason]
    REASON_UNSPECIFIED: SnapshotUnavailable.Reason
    REASON_MISSING: SnapshotUnavailable.Reason
    REASON_STALE: SnapshotUnavailable.Reason
    NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CHANGED_FIELD_NUMBER: _ClassVar[int]
    name: str
    reason: SnapshotUnavailable.Reason
    changed: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., reason: _Optional[_Union[SnapshotUnavailable.Reason, str]] = ..., changed: _Optional[_Iterable[str]] = ...) -> None: ...
