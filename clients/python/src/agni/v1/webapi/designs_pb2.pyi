import datetime

from agni.v1.webapi import checks_pb2 as _checks_pb2
from agni.v1.webapi import project_pb2 as _project_pb2
from agni.v1.webapi import snapshot_pb2 as _snapshot_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ArtifactType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ARTIFACT_TYPE_UNSPECIFIED: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_NETLIST: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_SCHEMATIC: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_BOARD: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_OTHER: _ClassVar[ArtifactType]
ARTIFACT_TYPE_UNSPECIFIED: ArtifactType
ARTIFACT_TYPE_NETLIST: ArtifactType
ARTIFACT_TYPE_SCHEMATIC: ArtifactType
ARTIFACT_TYPE_BOARD: ArtifactType
ARTIFACT_TYPE_OTHER: ArtifactType

class Report(_message.Message):
    __slots__ = ("name", "packs", "project", "design_hash", "pack_hashes", "results", "create_time", "design_changed", "packs_changed")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PACKS_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    DESIGN_HASH_FIELD_NUMBER: _ClassVar[int]
    PACK_HASHES_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    DESIGN_CHANGED_FIELD_NUMBER: _ClassVar[int]
    PACKS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    name: str
    packs: _containers.RepeatedScalarFieldContainer[str]
    project: str
    design_hash: str
    pack_hashes: _containers.RepeatedScalarFieldContainer[str]
    results: _checks_pb2.CheckDesignResponse
    create_time: _timestamp_pb2.Timestamp
    design_changed: bool
    packs_changed: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., packs: _Optional[_Iterable[str]] = ..., project: _Optional[str] = ..., design_hash: _Optional[str] = ..., pack_hashes: _Optional[_Iterable[str]] = ..., results: _Optional[_Union[_checks_pb2.CheckDesignResponse, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., design_changed: _Optional[bool] = ..., packs_changed: _Optional[_Iterable[str]] = ...) -> None: ...

class CreateReportRequest(_message.Message):
    __slots__ = ("parent", "report_id", "report", "project")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    REPORT_ID_FIELD_NUMBER: _ClassVar[int]
    REPORT_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    parent: str
    report_id: str
    report: Report
    project: str
    def __init__(self, parent: _Optional[str] = ..., report_id: _Optional[str] = ..., report: _Optional[_Union[Report, _Mapping]] = ..., project: _Optional[str] = ...) -> None: ...

class GetReportRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListReportsRequest(_message.Message):
    __slots__ = ("parent", "page_size", "page_token", "filter")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    parent: str
    page_size: int
    page_token: str
    filter: str
    def __init__(self, parent: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ...) -> None: ...

class ListReportsResponse(_message.Message):
    __slots__ = ("reports", "next_page_token")
    REPORTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    reports: _containers.RepeatedCompositeFieldContainer[Report]
    next_page_token: str
    def __init__(self, reports: _Optional[_Iterable[_Union[Report, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class DeleteReportRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateDesignRequest(_message.Message):
    __slots__ = ("design_id", "design")
    DESIGN_ID_FIELD_NUMBER: _ClassVar[int]
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    design_id: str
    design: _project_pb2.Design
    def __init__(self, design_id: _Optional[str] = ..., design: _Optional[_Union[_project_pb2.Design, _Mapping]] = ...) -> None: ...

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
    __slots__ = ("design", "update_mask")
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    design: _project_pb2.Design
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, design: _Optional[_Union[_project_pb2.Design, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class DeleteDesignRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class Artifact(_message.Message):
    __slots__ = ("name", "files", "type", "entry", "update_time")
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    name: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    type: ArtifactType
    entry: str
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, name: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ..., type: _Optional[_Union[ArtifactType, str]] = ..., entry: _Optional[str] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetArtifactRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListArtifactsRequest(_message.Message):
    __slots__ = ("parent", "page_size", "page_token")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    parent: str
    page_size: int
    page_token: str
    def __init__(self, parent: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListArtifactsResponse(_message.Message):
    __slots__ = ("artifacts", "next_page_token")
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    artifacts: _containers.RepeatedCompositeFieldContainer[Artifact]
    next_page_token: str
    def __init__(self, artifacts: _Optional[_Iterable[_Union[Artifact, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class UpdateArtifactRequest(_message.Message):
    __slots__ = ("artifact", "allow_missing")
    ARTIFACT_FIELD_NUMBER: _ClassVar[int]
    ALLOW_MISSING_FIELD_NUMBER: _ClassVar[int]
    artifact: Artifact
    allow_missing: bool
    def __init__(self, artifact: _Optional[_Union[Artifact, _Mapping]] = ..., allow_missing: _Optional[bool] = ...) -> None: ...

class BatchUpdateArtifactsRequest(_message.Message):
    __slots__ = ("parent", "requests")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    REQUESTS_FIELD_NUMBER: _ClassVar[int]
    parent: str
    requests: _containers.RepeatedCompositeFieldContainer[UpdateArtifactRequest]
    def __init__(self, parent: _Optional[str] = ..., requests: _Optional[_Iterable[_Union[UpdateArtifactRequest, _Mapping]]] = ...) -> None: ...

class BatchUpdateArtifactsResponse(_message.Message):
    __slots__ = ("artifacts",)
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    artifacts: _containers.RepeatedCompositeFieldContainer[Artifact]
    def __init__(self, artifacts: _Optional[_Iterable[_Union[Artifact, _Mapping]]] = ...) -> None: ...

class DeleteArtifactRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ImportDesignFilesRequest(_message.Message):
    __slots__ = ("parent", "files")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    parent: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    def __init__(self, parent: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ...) -> None: ...

class ImportDesignFilesResponse(_message.Message):
    __slots__ = ("artifacts", "unread")
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    UNREAD_FIELD_NUMBER: _ClassVar[int]
    artifacts: _containers.RepeatedCompositeFieldContainer[Artifact]
    unread: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, artifacts: _Optional[_Iterable[_Union[Artifact, _Mapping]]] = ..., unread: _Optional[_Iterable[str]] = ...) -> None: ...
