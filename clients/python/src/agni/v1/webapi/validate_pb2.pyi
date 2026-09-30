from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ValidateReport(_message.Message):
    __slots__ = ("files", "passed", "failed", "skipped")
    FILES_FIELD_NUMBER: _ClassVar[int]
    PASSED_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    files: _containers.RepeatedCompositeFieldContainer[FileValidation]
    passed: int
    failed: int
    skipped: int
    def __init__(self, files: _Optional[_Iterable[_Union[FileValidation, _Mapping]]] = ..., passed: _Optional[int] = ..., failed: _Optional[int] = ..., skipped: _Optional[int] = ...) -> None: ...

class FileValidation(_message.Message):
    __slots__ = ("path", "format", "ok", "problems", "netlist", "geometry")
    PATH_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    OK_FIELD_NUMBER: _ClassVar[int]
    PROBLEMS_FIELD_NUMBER: _ClassVar[int]
    NETLIST_FIELD_NUMBER: _ClassVar[int]
    GEOMETRY_FIELD_NUMBER: _ClassVar[int]
    path: str
    format: str
    ok: bool
    problems: _containers.RepeatedScalarFieldContainer[str]
    netlist: NetlistCounts
    geometry: GeometryCounts
    def __init__(self, path: _Optional[str] = ..., format: _Optional[str] = ..., ok: _Optional[bool] = ..., problems: _Optional[_Iterable[str]] = ..., netlist: _Optional[_Union[NetlistCounts, _Mapping]] = ..., geometry: _Optional[_Union[GeometryCounts, _Mapping]] = ...) -> None: ...

class NetlistCounts(_message.Message):
    __slots__ = ("components", "nets")
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    components: int
    nets: int
    def __init__(self, components: _Optional[int] = ..., nets: _Optional[int] = ...) -> None: ...

class GeometryCounts(_message.Message):
    __slots__ = ("sheets", "symbols", "placements", "wires", "resolved")
    SHEETS_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    PLACEMENTS_FIELD_NUMBER: _ClassVar[int]
    WIRES_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_FIELD_NUMBER: _ClassVar[int]
    sheets: int
    symbols: int
    placements: int
    wires: int
    resolved: int
    def __init__(self, sheets: _Optional[int] = ..., symbols: _Optional[int] = ..., placements: _Optional[int] = ..., wires: _Optional[int] = ..., resolved: _Optional[int] = ...) -> None: ...
