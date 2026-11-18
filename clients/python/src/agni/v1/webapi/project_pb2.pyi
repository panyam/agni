import datetime

from agni.v1.webapi import config_pb2 as _config_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Project(_message.Message):
    __slots__ = ("name", "title", "uri", "config", "designs", "default_packs")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_PACKS_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    uri: str
    config: _config_pb2.AnalysisConfig
    designs: _containers.RepeatedScalarFieldContainer[str]
    default_packs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., uri: _Optional[str] = ..., config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ..., designs: _Optional[_Iterable[str]] = ..., default_packs: _Optional[_Iterable[str]] = ...) -> None: ...

class Design(_message.Message):
    __slots__ = ("name", "title", "uri", "entry_uri", "companion_uris", "config", "series", "flatten_hierarchy", "update_time", "series_latest", "artifacts", "read_only")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    ENTRY_URI_FIELD_NUMBER: _ClassVar[int]
    COMPANION_URIS_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    FLATTEN_HIERARCHY_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    SERIES_LATEST_FIELD_NUMBER: _ClassVar[int]
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    READ_ONLY_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    uri: str
    entry_uri: str
    companion_uris: _containers.RepeatedScalarFieldContainer[str]
    config: _config_pb2.AnalysisConfig
    series: str
    flatten_hierarchy: bool
    update_time: _timestamp_pb2.Timestamp
    series_latest: bool
    artifacts: _containers.RepeatedScalarFieldContainer[str]
    read_only: bool
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., uri: _Optional[str] = ..., entry_uri: _Optional[str] = ..., companion_uris: _Optional[_Iterable[str]] = ..., config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ..., series: _Optional[str] = ..., flatten_hierarchy: _Optional[bool] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., series_latest: _Optional[bool] = ..., artifacts: _Optional[_Iterable[str]] = ..., read_only: _Optional[bool] = ...) -> None: ...

class GetProjectRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListProjectsRequest(_message.Message):
    __slots__ = ("page_size", "page_token", "filter")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    filter: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., filter: _Optional[str] = ...) -> None: ...

class ListProjectsResponse(_message.Message):
    __slots__ = ("projects", "next_page_token")
    PROJECTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    projects: _containers.RepeatedCompositeFieldContainer[Project]
    next_page_token: str
    def __init__(self, projects: _Optional[_Iterable[_Union[Project, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class CreateProjectRequest(_message.Message):
    __slots__ = ("project_id", "project")
    PROJECT_ID_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    project_id: str
    project: Project
    def __init__(self, project_id: _Optional[str] = ..., project: _Optional[_Union[Project, _Mapping]] = ...) -> None: ...

class UpdateProjectRequest(_message.Message):
    __slots__ = ("project", "update_mask")
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    project: Project
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, project: _Optional[_Union[Project, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class DeleteProjectRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ResolveDesignRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class ResolveDesignResponse(_message.Message):
    __slots__ = ("design", "project")
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    design: Design
    project: Project
    def __init__(self, design: _Optional[_Union[Design, _Mapping]] = ..., project: _Optional[_Union[Project, _Mapping]] = ...) -> None: ...
