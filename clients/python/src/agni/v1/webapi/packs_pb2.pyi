import datetime

from agni.v1.webapi import config_pb2 as _config_pb2
from agni.v1.webapi import designs_pb2 as _designs_pb2
from agni.v1.webapi import snapshot_pb2 as _snapshot_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LinkPackRequest(_message.Message):
    __slots__ = ("pack_id", "source", "title")
    PACK_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    pack_id: str
    source: _designs_pb2.ArtifactSource
    title: str
    def __init__(self, pack_id: _Optional[str] = ..., source: _Optional[_Union[_designs_pb2.ArtifactSource, _Mapping]] = ..., title: _Optional[str] = ...) -> None: ...

class UnlinkPackRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class SyncPackRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class SyncPackResponse(_message.Message):
    __slots__ = ("pack", "from_version", "to_version", "changed")
    PACK_FIELD_NUMBER: _ClassVar[int]
    FROM_VERSION_FIELD_NUMBER: _ClassVar[int]
    TO_VERSION_FIELD_NUMBER: _ClassVar[int]
    CHANGED_FIELD_NUMBER: _ClassVar[int]
    pack: Pack
    from_version: str
    to_version: str
    changed: bool
    def __init__(self, pack: _Optional[_Union[Pack, _Mapping]] = ..., from_version: _Optional[str] = ..., to_version: _Optional[str] = ..., changed: _Optional[bool] = ...) -> None: ...

class ImportPackFilesRequest(_message.Message):
    __slots__ = ("pack_id", "files", "title")
    PACK_ID_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    pack_id: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    title: str
    def __init__(self, pack_id: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ..., title: _Optional[str] = ...) -> None: ...

class Pack(_message.Message):
    __slots__ = ("name", "title", "config", "files", "uri", "hash", "update_time", "source")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    HASH_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    config: _config_pb2.AnalysisConfig
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    uri: str
    hash: str
    update_time: _timestamp_pb2.Timestamp
    source: _designs_pb2.ArtifactSource
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ..., uri: _Optional[str] = ..., hash: _Optional[str] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., source: _Optional[_Union[_designs_pb2.ArtifactSource, _Mapping]] = ...) -> None: ...

class CreatePackRequest(_message.Message):
    __slots__ = ("pack_id", "pack")
    PACK_ID_FIELD_NUMBER: _ClassVar[int]
    PACK_FIELD_NUMBER: _ClassVar[int]
    pack_id: str
    pack: Pack
    def __init__(self, pack_id: _Optional[str] = ..., pack: _Optional[_Union[Pack, _Mapping]] = ...) -> None: ...

class GetPackRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListPacksRequest(_message.Message):
    __slots__ = ("page_size", "page_token", "filter")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    filter: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ...) -> None: ...

class ListPacksResponse(_message.Message):
    __slots__ = ("packs", "next_page_token")
    PACKS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    packs: _containers.RepeatedCompositeFieldContainer[Pack]
    next_page_token: str
    def __init__(self, packs: _Optional[_Iterable[_Union[Pack, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class UpdatePackRequest(_message.Message):
    __slots__ = ("pack", "update_mask")
    PACK_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    pack: Pack
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, pack: _Optional[_Union[Pack, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class DeletePackRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...
