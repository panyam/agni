from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.webapi import checks_pb2 as _checks_pb2_1
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Review(_message.Message):
    __slots__ = ("name", "results")
    NAME_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    name: str
    results: _checks_pb2.CheckResults
    def __init__(self, name: _Optional[str] = ..., results: _Optional[_Union[_checks_pb2.CheckResults, _Mapping]] = ...) -> None: ...

class CreateReviewRequest(_message.Message):
    __slots__ = ("parent", "design_uri", "board_uri", "ratified_floor", "overlay", "manifest", "as_named")
    PARENT_FIELD_NUMBER: _ClassVar[int]
    DESIGN_URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    RATIFIED_FLOOR_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    parent: str
    design_uri: str
    board_uri: str
    ratified_floor: float
    overlay: _checks_pb2_1.OverlayConfig
    manifest: _checks_pb2.ReviewManifest
    as_named: bool
    def __init__(self, parent: _Optional[str] = ..., design_uri: _Optional[str] = ..., board_uri: _Optional[str] = ..., ratified_floor: _Optional[float] = ..., overlay: _Optional[_Union[_checks_pb2_1.OverlayConfig, _Mapping]] = ..., manifest: _Optional[_Union[_checks_pb2.ReviewManifest, _Mapping]] = ..., as_named: _Optional[bool] = ...) -> None: ...

class GetReviewRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListReviewsRequest(_message.Message):
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

class ListReviewsResponse(_message.Message):
    __slots__ = ("reviews", "next_page_token")
    REVIEWS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    reviews: _containers.RepeatedCompositeFieldContainer[Review]
    next_page_token: str
    def __init__(self, reviews: _Optional[_Iterable[_Union[Review, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class DeleteReviewRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetReviewManifestRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class GetReviewManifestResponse(_message.Message):
    __slots__ = ("manifest",)
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    manifest: _checks_pb2.ReviewManifest
    def __init__(self, manifest: _Optional[_Union[_checks_pb2.ReviewManifest, _Mapping]] = ...) -> None: ...
