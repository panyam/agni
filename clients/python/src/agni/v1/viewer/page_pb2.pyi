from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ViewerLocation(_message.Message):
    __slots__ = ("mount", "path", "sheet", "mode", "layout", "symbols", "flatten", "verdict", "hash", "rule", "subjects", "trace", "trace_hops", "query", "title", "binds", "set", "q", "against", "rename_approx")
    MOUNT_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    SHEET_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    FLATTEN_FIELD_NUMBER: _ClassVar[int]
    VERDICT_FIELD_NUMBER: _ClassVar[int]
    HASH_FIELD_NUMBER: _ClassVar[int]
    RULE_FIELD_NUMBER: _ClassVar[int]
    SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    TRACE_FIELD_NUMBER: _ClassVar[int]
    TRACE_HOPS_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    BINDS_FIELD_NUMBER: _ClassVar[int]
    SET_FIELD_NUMBER: _ClassVar[int]
    Q_FIELD_NUMBER: _ClassVar[int]
    AGAINST_FIELD_NUMBER: _ClassVar[int]
    RENAME_APPROX_FIELD_NUMBER: _ClassVar[int]
    mount: str
    path: str
    sheet: str
    mode: str
    layout: str
    symbols: bool
    flatten: bool
    verdict: str
    hash: str
    rule: str
    subjects: _containers.RepeatedScalarFieldContainer[str]
    trace: str
    trace_hops: int
    query: str
    title: str
    binds: _containers.RepeatedScalarFieldContainer[str]
    set: str
    q: str
    against: str
    rename_approx: bool
    def __init__(self, mount: _Optional[str] = ..., path: _Optional[str] = ..., sheet: _Optional[str] = ..., mode: _Optional[str] = ..., layout: _Optional[str] = ..., symbols: _Optional[bool] = ..., flatten: _Optional[bool] = ..., verdict: _Optional[str] = ..., hash: _Optional[str] = ..., rule: _Optional[str] = ..., subjects: _Optional[_Iterable[str]] = ..., trace: _Optional[str] = ..., trace_hops: _Optional[int] = ..., query: _Optional[str] = ..., title: _Optional[str] = ..., binds: _Optional[_Iterable[str]] = ..., set: _Optional[str] = ..., q: _Optional[str] = ..., against: _Optional[str] = ..., rename_approx: _Optional[bool] = ...) -> None: ...

class GetLocationRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetLocationResponse(_message.Message):
    __slots__ = ("location",)
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    location: ViewerLocation
    def __init__(self, location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...

class ApplyLocationRequest(_message.Message):
    __slots__ = ("location",)
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    location: ViewerLocation
    def __init__(self, location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...

class ApplyLocationResponse(_message.Message):
    __slots__ = ("location", "verdict_resolved")
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    VERDICT_RESOLVED_FIELD_NUMBER: _ClassVar[int]
    location: ViewerLocation
    verdict_resolved: bool
    def __init__(self, location: _Optional[_Union[ViewerLocation, _Mapping]] = ..., verdict_resolved: _Optional[bool] = ...) -> None: ...

class LocateVerdictRequest(_message.Message):
    __slots__ = ("id", "rule", "subjects")
    ID_FIELD_NUMBER: _ClassVar[int]
    RULE_FIELD_NUMBER: _ClassVar[int]
    SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    id: str
    rule: str
    subjects: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., rule: _Optional[str] = ..., subjects: _Optional[_Iterable[str]] = ...) -> None: ...

class LocateVerdictResponse(_message.Message):
    __slots__ = ("resolved", "location")
    RESOLVED_FIELD_NUMBER: _ClassVar[int]
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    resolved: bool
    location: ViewerLocation
    def __init__(self, resolved: _Optional[bool] = ..., location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...

class SelectFindingRequest(_message.Message):
    __slots__ = ("subject", "net_id", "sheet")
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    NET_ID_FIELD_NUMBER: _ClassVar[int]
    SHEET_FIELD_NUMBER: _ClassVar[int]
    subject: str
    net_id: str
    sheet: str
    def __init__(self, subject: _Optional[str] = ..., net_id: _Optional[str] = ..., sheet: _Optional[str] = ...) -> None: ...

class SelectFindingResponse(_message.Message):
    __slots__ = ("selected", "location")
    SELECTED_FIELD_NUMBER: _ClassVar[int]
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    selected: bool
    location: ViewerLocation
    def __init__(self, selected: _Optional[bool] = ..., location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...

class LocateEntityRequest(_message.Message):
    __slots__ = ("kind", "subject", "pin", "sheet")
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    SHEET_FIELD_NUMBER: _ClassVar[int]
    kind: str
    subject: str
    pin: str
    sheet: str
    def __init__(self, kind: _Optional[str] = ..., subject: _Optional[str] = ..., pin: _Optional[str] = ..., sheet: _Optional[str] = ...) -> None: ...

class LocateEntityResponse(_message.Message):
    __slots__ = ("location",)
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    location: ViewerLocation
    def __init__(self, location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...

class ShowSheetRequest(_message.Message):
    __slots__ = ("sheet",)
    SHEET_FIELD_NUMBER: _ClassVar[int]
    sheet: str
    def __init__(self, sheet: _Optional[str] = ...) -> None: ...

class ShowSheetResponse(_message.Message):
    __slots__ = ("location",)
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    location: ViewerLocation
    def __init__(self, location: _Optional[_Union[ViewerLocation, _Mapping]] = ...) -> None: ...
