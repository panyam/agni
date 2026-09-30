from agni.v1.webapi import config_pb2 as _config_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Project(_message.Message):
    __slots__ = ("name", "title", "uri", "config")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    uri: str
    config: _config_pb2.AnalysisConfig
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., uri: _Optional[str] = ..., config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ...) -> None: ...

class Design(_message.Message):
    __slots__ = ("name", "title", "uri", "entry_uri", "companion_uris", "config")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    ENTRY_URI_FIELD_NUMBER: _ClassVar[int]
    COMPANION_URIS_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    uri: str
    entry_uri: str
    companion_uris: _containers.RepeatedScalarFieldContainer[str]
    config: _config_pb2.AnalysisConfig
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., uri: _Optional[str] = ..., entry_uri: _Optional[str] = ..., companion_uris: _Optional[_Iterable[str]] = ..., config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ...) -> None: ...

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

class GetProjectDesignRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListProjectDesignsRequest(_message.Message):
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

class ListProjectDesignsResponse(_message.Message):
    __slots__ = ("designs", "next_page_token")
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    designs: _containers.RepeatedCompositeFieldContainer[Design]
    next_page_token: str
    def __init__(self, designs: _Optional[_Iterable[_Union[Design, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

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
