from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.webapi import checks_pb2 as _checks_pb2_1
from agni.v1.webapi import config_pb2 as _config_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RenderReviewReportRequest(_message.Message):
    __slots__ = ("review", "links")
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    LINKS_FIELD_NUMBER: _ClassVar[int]
    review: Review
    links: _checks_pb2_1.ReportLinks
    def __init__(self, review: _Optional[_Union[Review, _Mapping]] = ..., links: _Optional[_Union[_checks_pb2_1.ReportLinks, _Mapping]] = ...) -> None: ...

class RenderReviewReportResponse(_message.Message):
    __slots__ = ("content", "content_type", "filename")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    content: bytes
    content_type: str
    filename: str
    def __init__(self, content: _Optional[bytes] = ..., content_type: _Optional[str] = ..., filename: _Optional[str] = ...) -> None: ...

class Review(_message.Message):
    __slots__ = ("name", "results", "summary")
    NAME_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    name: str
    results: _checks_pb2.CheckResults
    summary: ReviewSummary
    def __init__(self, name: _Optional[str] = ..., results: _Optional[_Union[_checks_pb2.CheckResults, _Mapping]] = ..., summary: _Optional[_Union[ReviewSummary, _Mapping]] = ...) -> None: ...

class ReviewSummary(_message.Message):
    __slots__ = ("total", "covered", "answered", "fail", "provisional")
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    COVERED_FIELD_NUMBER: _ClassVar[int]
    ANSWERED_FIELD_NUMBER: _ClassVar[int]
    PASS_FIELD_NUMBER: _ClassVar[int]
    FAIL_FIELD_NUMBER: _ClassVar[int]
    PROVISIONAL_FIELD_NUMBER: _ClassVar[int]
    total: int
    covered: int
    answered: int
    fail: int
    provisional: int
    def __init__(self, total: _Optional[int] = ..., covered: _Optional[int] = ..., answered: _Optional[int] = ..., fail: _Optional[int] = ..., provisional: _Optional[int] = ..., **kwargs) -> None: ...

class CreateReviewRequest(_message.Message):
    __slots__ = ("snapshot", "parent", "ratified_floor", "manifest", "work_budget", "validate_only")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    PARENT_FIELD_NUMBER: _ClassVar[int]
    RATIFIED_FLOOR_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    WORK_BUDGET_FIELD_NUMBER: _ClassVar[int]
    VALIDATE_ONLY_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    parent: str
    ratified_floor: float
    manifest: _checks_pb2.ReviewManifest
    work_budget: int
    validate_only: bool
    def __init__(self, snapshot: _Optional[str] = ..., parent: _Optional[str] = ..., ratified_floor: _Optional[float] = ..., manifest: _Optional[_Union[_checks_pb2.ReviewManifest, _Mapping]] = ..., work_budget: _Optional[int] = ..., validate_only: _Optional[bool] = ...) -> None: ...

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

class ListChecklistsRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class ListChecklistsResponse(_message.Message):
    __slots__ = ("project", "checklists")
    PROJECT_FIELD_NUMBER: _ClassVar[int]
    CHECKLISTS_FIELD_NUMBER: _ClassVar[int]
    project: str
    checklists: _containers.RepeatedCompositeFieldContainer[_config_pb2.NamedChecklist]
    def __init__(self, project: _Optional[str] = ..., checklists: _Optional[_Iterable[_Union[_config_pb2.NamedChecklist, _Mapping]]] = ...) -> None: ...
