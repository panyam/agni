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

class DesignRevisionSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DESIGN_REVISION_SOURCE_UNSPECIFIED: _ClassVar[DesignRevisionSource]
    DESIGN_REVISION_SOURCE_TAGS: _ClassVar[DesignRevisionSource]
    DESIGN_REVISION_SOURCE_COMMITS: _ClassVar[DesignRevisionSource]

class ReportView(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REPORT_VIEW_UNSPECIFIED: _ClassVar[ReportView]
    REPORT_VIEW_BASIC: _ClassVar[ReportView]
    REPORT_VIEW_FULL: _ClassVar[ReportView]

class OnExists(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ON_EXISTS_UNSPECIFIED: _ClassVar[OnExists]
    ON_EXISTS_FAIL: _ClassVar[OnExists]
    ON_EXISTS_SKIP: _ClassVar[OnExists]
    ON_EXISTS_UPDATE: _ClassVar[OnExists]

class AddOutcome(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ADD_OUTCOME_UNSPECIFIED: _ClassVar[AddOutcome]
    ADD_OUTCOME_ADDED: _ClassVar[AddOutcome]
    ADD_OUTCOME_UPDATED: _ClassVar[AddOutcome]
    ADD_OUTCOME_UNCHANGED: _ClassVar[AddOutcome]
    ADD_OUTCOME_SKIPPED: _ClassVar[AddOutcome]
    ADD_OUTCOME_REFUSED: _ClassVar[AddOutcome]

class ArtifactType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ARTIFACT_TYPE_UNSPECIFIED: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_NETLIST: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_SCHEMATIC: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_BOARD: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_OTHER: _ClassVar[ArtifactType]
    ARTIFACT_TYPE_SYMBOLS: _ClassVar[ArtifactType]
DESIGN_REVISION_SOURCE_UNSPECIFIED: DesignRevisionSource
DESIGN_REVISION_SOURCE_TAGS: DesignRevisionSource
DESIGN_REVISION_SOURCE_COMMITS: DesignRevisionSource
REPORT_VIEW_UNSPECIFIED: ReportView
REPORT_VIEW_BASIC: ReportView
REPORT_VIEW_FULL: ReportView
ON_EXISTS_UNSPECIFIED: OnExists
ON_EXISTS_FAIL: OnExists
ON_EXISTS_SKIP: OnExists
ON_EXISTS_UPDATE: OnExists
ADD_OUTCOME_UNSPECIFIED: AddOutcome
ADD_OUTCOME_ADDED: AddOutcome
ADD_OUTCOME_UPDATED: AddOutcome
ADD_OUTCOME_UNCHANGED: AddOutcome
ADD_OUTCOME_SKIPPED: AddOutcome
ADD_OUTCOME_REFUSED: AddOutcome
ARTIFACT_TYPE_UNSPECIFIED: ArtifactType
ARTIFACT_TYPE_NETLIST: ArtifactType
ARTIFACT_TYPE_SCHEMATIC: ArtifactType
ARTIFACT_TYPE_BOARD: ArtifactType
ARTIFACT_TYPE_OTHER: ArtifactType
ARTIFACT_TYPE_SYMBOLS: ArtifactType

class Report(_message.Message):
    __slots__ = ("name", "packs", "project", "design_hash", "pack_hashes", "results", "create_time", "design_changed", "packs_changed", "finding_count", "source_versions", "revision", "pack_versions")
    class SourceVersionsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class PackVersionsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    PACKS_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    DESIGN_HASH_FIELD_NUMBER: _ClassVar[int]
    PACK_HASHES_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    DESIGN_CHANGED_FIELD_NUMBER: _ClassVar[int]
    PACKS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    FINDING_COUNT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_VERSIONS_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    PACK_VERSIONS_FIELD_NUMBER: _ClassVar[int]
    name: str
    packs: _containers.RepeatedScalarFieldContainer[str]
    project: str
    design_hash: str
    pack_hashes: _containers.RepeatedScalarFieldContainer[str]
    results: _checks_pb2.CheckDesignResponse
    create_time: _timestamp_pb2.Timestamp
    design_changed: bool
    packs_changed: _containers.RepeatedScalarFieldContainer[str]
    finding_count: int
    source_versions: _containers.ScalarMap[str, str]
    revision: str
    pack_versions: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., packs: _Optional[_Iterable[str]] = ..., project: _Optional[str] = ..., design_hash: _Optional[str] = ..., pack_hashes: _Optional[_Iterable[str]] = ..., results: _Optional[_Union[_checks_pb2.CheckDesignResponse, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., design_changed: _Optional[bool] = ..., packs_changed: _Optional[_Iterable[str]] = ..., finding_count: _Optional[int] = ..., source_versions: _Optional[_Mapping[str, str]] = ..., revision: _Optional[str] = ..., pack_versions: _Optional[_Mapping[str, str]] = ...) -> None: ...

class DesignRevision(_message.Message):
    __slots__ = ("name", "revision_id", "refs", "source_versions", "create_time", "current", "commit_time", "message")
    class SourceVersionsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    REVISION_ID_FIELD_NUMBER: _ClassVar[int]
    REFS_FIELD_NUMBER: _ClassVar[int]
    SOURCE_VERSIONS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    CURRENT_FIELD_NUMBER: _ClassVar[int]
    COMMIT_TIME_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    name: str
    revision_id: str
    refs: _containers.RepeatedScalarFieldContainer[str]
    source_versions: _containers.ScalarMap[str, str]
    create_time: _timestamp_pb2.Timestamp
    current: bool
    commit_time: _timestamp_pb2.Timestamp
    message: str
    def __init__(self, name: _Optional[str] = ..., revision_id: _Optional[str] = ..., refs: _Optional[_Iterable[str]] = ..., source_versions: _Optional[_Mapping[str, str]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., current: _Optional[bool] = ..., commit_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., message: _Optional[str] = ...) -> None: ...

class ListDesignRevisionsRequest(_message.Message):
    __slots__ = ("parent", "page_size", "page_token", "source", "branch")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    BRANCH_FIELD_NUMBER: _ClassVar[int]
    parent: str
    page_size: int
    page_token: str
    source: DesignRevisionSource
    branch: str
    def __init__(self, parent: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., source: _Optional[_Union[DesignRevisionSource, str]] = ..., branch: _Optional[str] = ...) -> None: ...

class ListDesignRevisionsResponse(_message.Message):
    __slots__ = ("revisions", "next_page_token")
    REVISIONS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    revisions: _containers.RepeatedCompositeFieldContainer[DesignRevision]
    next_page_token: str
    def __init__(self, revisions: _Optional[_Iterable[_Union[DesignRevision, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

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
    __slots__ = ("name", "view")
    NAME_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    name: str
    view: ReportView
    def __init__(self, name: _Optional[str] = ..., view: _Optional[_Union[ReportView, str]] = ...) -> None: ...

class ListReportsRequest(_message.Message):
    __slots__ = ("parent", "page_size", "page_token", "filter", "view")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    parent: str
    page_size: int
    page_token: str
    filter: str
    view: ReportView
    def __init__(self, parent: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ..., view: _Optional[_Union[ReportView, str]] = ...) -> None: ...

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
    __slots__ = ("name", "files", "type", "entry", "update_time", "source")
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    name: str
    files: _containers.RepeatedCompositeFieldContainer[_snapshot_pb2.SnapshotFile]
    type: ArtifactType
    entry: str
    update_time: _timestamp_pb2.Timestamp
    source: ArtifactSource
    def __init__(self, name: _Optional[str] = ..., files: _Optional[_Iterable[_Union[_snapshot_pb2.SnapshotFile, _Mapping]]] = ..., type: _Optional[_Union[ArtifactType, str]] = ..., entry: _Optional[str] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., source: _Optional[_Union[ArtifactSource, _Mapping]] = ...) -> None: ...

class ArtifactSource(_message.Message):
    __slots__ = ("mount", "path", "ref", "follow", "version", "read_time", "read_hash", "access_source", "newer_version", "newer_time")
    MOUNT_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    REF_FIELD_NUMBER: _ClassVar[int]
    FOLLOW_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    READ_TIME_FIELD_NUMBER: _ClassVar[int]
    READ_HASH_FIELD_NUMBER: _ClassVar[int]
    ACCESS_SOURCE_FIELD_NUMBER: _ClassVar[int]
    NEWER_VERSION_FIELD_NUMBER: _ClassVar[int]
    NEWER_TIME_FIELD_NUMBER: _ClassVar[int]
    mount: str
    path: str
    ref: str
    follow: bool
    version: str
    read_time: _timestamp_pb2.Timestamp
    read_hash: str
    access_source: str
    newer_version: str
    newer_time: _timestamp_pb2.Timestamp
    def __init__(self, mount: _Optional[str] = ..., path: _Optional[str] = ..., ref: _Optional[str] = ..., follow: _Optional[bool] = ..., version: _Optional[str] = ..., read_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., read_hash: _Optional[str] = ..., access_source: _Optional[str] = ..., newer_version: _Optional[str] = ..., newer_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AddDesignsRequest(_message.Message):
    __slots__ = ("uri", "designs", "validate_only", "no_symbol_copy", "on_exists", "allow_partial")
    URI_FIELD_NUMBER: _ClassVar[int]
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    VALIDATE_ONLY_FIELD_NUMBER: _ClassVar[int]
    NO_SYMBOL_COPY_FIELD_NUMBER: _ClassVar[int]
    ON_EXISTS_FIELD_NUMBER: _ClassVar[int]
    ALLOW_PARTIAL_FIELD_NUMBER: _ClassVar[int]
    uri: str
    designs: _containers.RepeatedCompositeFieldContainer[DesignToAdd]
    validate_only: bool
    no_symbol_copy: bool
    on_exists: OnExists
    allow_partial: bool
    def __init__(self, uri: _Optional[str] = ..., designs: _Optional[_Iterable[_Union[DesignToAdd, _Mapping]]] = ..., validate_only: _Optional[bool] = ..., no_symbol_copy: _Optional[bool] = ..., on_exists: _Optional[_Union[OnExists, str]] = ..., allow_partial: _Optional[bool] = ...) -> None: ...

class AddResult(_message.Message):
    __slots__ = ("folder", "entry", "design", "outcome", "reason")
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    OUTCOME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    folder: str
    entry: str
    design: str
    outcome: AddOutcome
    reason: str
    def __init__(self, folder: _Optional[str] = ..., entry: _Optional[str] = ..., design: _Optional[str] = ..., outcome: _Optional[_Union[AddOutcome, str]] = ..., reason: _Optional[str] = ...) -> None: ...

class DesignToAdd(_message.Message):
    __slots__ = ("folder", "design_id", "title", "series", "entry")
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    DESIGN_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    folder: str
    design_id: str
    title: str
    series: str
    entry: str
    def __init__(self, folder: _Optional[str] = ..., design_id: _Optional[str] = ..., title: _Optional[str] = ..., series: _Optional[str] = ..., entry: _Optional[str] = ...) -> None: ...

class AddDesignsResponse(_message.Message):
    __slots__ = ("designs", "uncopied_symbols", "results")
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    UNCOPIED_SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    designs: _containers.RepeatedCompositeFieldContainer[_project_pb2.Design]
    uncopied_symbols: _containers.RepeatedScalarFieldContainer[str]
    results: _containers.RepeatedCompositeFieldContainer[AddResult]
    def __init__(self, designs: _Optional[_Iterable[_Union[_project_pb2.Design, _Mapping]]] = ..., uncopied_symbols: _Optional[_Iterable[str]] = ..., results: _Optional[_Iterable[_Union[AddResult, _Mapping]]] = ...) -> None: ...

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

class LinkArtifactRequest(_message.Message):
    __slots__ = ("name", "source", "allow_missing")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ALLOW_MISSING_FIELD_NUMBER: _ClassVar[int]
    name: str
    source: ArtifactSource
    allow_missing: bool
    def __init__(self, name: _Optional[str] = ..., source: _Optional[_Union[ArtifactSource, _Mapping]] = ..., allow_missing: _Optional[bool] = ...) -> None: ...

class UnlinkArtifactRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class SyncDesignRequest(_message.Message):
    __slots__ = ("name", "artifacts", "validate_only")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    VALIDATE_ONLY_FIELD_NUMBER: _ClassVar[int]
    name: str
    artifacts: _containers.RepeatedScalarFieldContainer[str]
    validate_only: bool
    def __init__(self, name: _Optional[str] = ..., artifacts: _Optional[_Iterable[str]] = ..., validate_only: _Optional[bool] = ...) -> None: ...

class SyncDesignResponse(_message.Message):
    __slots__ = ("artifacts",)
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    artifacts: _containers.RepeatedCompositeFieldContainer[ArtifactSync]
    def __init__(self, artifacts: _Optional[_Iterable[_Union[ArtifactSync, _Mapping]]] = ...) -> None: ...

class ArtifactSync(_message.Message):
    __slots__ = ("artifact", "from_version", "to_version", "changed", "files_fetched", "bytes_fetched")
    ARTIFACT_FIELD_NUMBER: _ClassVar[int]
    FROM_VERSION_FIELD_NUMBER: _ClassVar[int]
    TO_VERSION_FIELD_NUMBER: _ClassVar[int]
    CHANGED_FIELD_NUMBER: _ClassVar[int]
    FILES_FETCHED_FIELD_NUMBER: _ClassVar[int]
    BYTES_FETCHED_FIELD_NUMBER: _ClassVar[int]
    artifact: str
    from_version: str
    to_version: str
    changed: bool
    files_fetched: int
    bytes_fetched: int
    def __init__(self, artifact: _Optional[str] = ..., from_version: _Optional[str] = ..., to_version: _Optional[str] = ..., changed: _Optional[bool] = ..., files_fetched: _Optional[int] = ..., bytes_fetched: _Optional[int] = ...) -> None: ...

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
