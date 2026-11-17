from agni.v1.webapi import project_pb2 as _project_pb2
from agni.v1.webapi import snapshot_pb2 as _snapshot_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateDesignRequest(_message.Message):
    __slots__ = ("design_id", "files")
    DESIGN_ID_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    design_id: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    def __init__(self, design_id: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ...) -> None: ...

class GetDesignRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListDesignsRequest(_message.Message):
    __slots__ = ("page_size", "page_token", "filter")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    filter: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ...) -> None: ...

class ListDesignsResponse(_message.Message):
    __slots__ = ("designs", "next_page_token", "unreachable")
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    UNREACHABLE_FIELD_NUMBER: _ClassVar[int]
    designs: _containers.RepeatedCompositeFieldContainer[_project_pb2.Design]
    next_page_token: str
    unreachable: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, designs: _Optional[_Iterable[_Union[_project_pb2.Design, _Mapping]]] = ..., next_page_token: _Optional[str] = ..., unreachable: _Optional[_Iterable[str]] = ...) -> None: ...

class UpdateDesignRequest(_message.Message):
    __slots__ = ("name", "files")
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    name: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    def __init__(self, name: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ...) -> None: ...

class DeleteDesignRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...
