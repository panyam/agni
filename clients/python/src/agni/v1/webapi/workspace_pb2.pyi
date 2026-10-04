from agni.v1.webapi import project_pb2 as _project_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class FileKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    FILE_KIND_UNSPECIFIED: _ClassVar[FileKind]
    FILE_KIND_DESIGN: _ClassVar[FileKind]
    FILE_KIND_DATASHEET: _ClassVar[FileKind]
FILE_KIND_UNSPECIFIED: FileKind
FILE_KIND_DESIGN: FileKind
FILE_KIND_DATASHEET: FileKind

class Mount(_message.Message):
    __slots__ = ("name", "root", "uri")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ROOT_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    name: str
    root: str
    uri: str
    def __init__(self, name: _Optional[str] = ..., root: _Optional[str] = ..., uri: _Optional[str] = ...) -> None: ...

class ListMountsRequest(_message.Message):
    __slots__ = ("opens",)
    OPENS_FIELD_NUMBER: _ClassVar[int]
    opens: _containers.RepeatedScalarFieldContainer[FileKind]
    def __init__(self, opens: _Optional[_Iterable[_Union[FileKind, str]]] = ...) -> None: ...

class ListMountsResponse(_message.Message):
    __slots__ = ("mounts", "pruned_mounts")
    MOUNTS_FIELD_NUMBER: _ClassVar[int]
    PRUNED_MOUNTS_FIELD_NUMBER: _ClassVar[int]
    mounts: _containers.RepeatedCompositeFieldContainer[Mount]
    pruned_mounts: int
    def __init__(self, mounts: _Optional[_Iterable[_Union[Mount, _Mapping]]] = ..., pruned_mounts: _Optional[int] = ...) -> None: ...

class DirEntry(_message.Message):
    __slots__ = ("name", "is_dir", "format", "uri", "kind")
    NAME_FIELD_NUMBER: _ClassVar[int]
    IS_DIR_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    name: str
    is_dir: bool
    format: str
    uri: str
    kind: FileKind
    def __init__(self, name: _Optional[str] = ..., is_dir: _Optional[bool] = ..., format: _Optional[str] = ..., uri: _Optional[str] = ..., kind: _Optional[_Union[FileKind, str]] = ...) -> None: ...

class ListDirRequest(_message.Message):
    __slots__ = ("uri", "opens")
    URI_FIELD_NUMBER: _ClassVar[int]
    OPENS_FIELD_NUMBER: _ClassVar[int]
    uri: str
    opens: _containers.RepeatedScalarFieldContainer[FileKind]
    def __init__(self, uri: _Optional[str] = ..., opens: _Optional[_Iterable[_Union[FileKind, str]]] = ...) -> None: ...

class ListDirResponse(_message.Message):
    __slots__ = ("entries",)
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[DirEntry]
    def __init__(self, entries: _Optional[_Iterable[_Union[DirEntry, _Mapping]]] = ...) -> None: ...

class ListDesignFilesRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class ListDesignFilesResponse(_message.Message):
    __slots__ = ("mount", "files", "total_size")
    MOUNT_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_SIZE_FIELD_NUMBER: _ClassVar[int]
    mount: str
    files: _containers.RepeatedCompositeFieldContainer[DesignFile]
    total_size: int
    def __init__(self, mount: _Optional[str] = ..., files: _Optional[_Iterable[_Union[DesignFile, _Mapping]]] = ..., total_size: _Optional[int] = ...) -> None: ...

class DesignFile(_message.Message):
    __slots__ = ("path", "size", "sha256")
    PATH_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    path: str
    size: int
    sha256: str
    def __init__(self, path: _Optional[str] = ..., size: _Optional[int] = ..., sha256: _Optional[str] = ...) -> None: ...

class ProposeDesignsRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class ProposeDesignsResponse(_message.Message):
    __slots__ = ("designs", "unread", "support")
    DESIGNS_FIELD_NUMBER: _ClassVar[int]
    UNREAD_FIELD_NUMBER: _ClassVar[int]
    SUPPORT_FIELD_NUMBER: _ClassVar[int]
    designs: _containers.RepeatedCompositeFieldContainer[ProposedDesign]
    unread: _containers.RepeatedCompositeFieldContainer[UnreadFile]
    support: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, designs: _Optional[_Iterable[_Union[ProposedDesign, _Mapping]]] = ..., unread: _Optional[_Iterable[_Union[UnreadFile, _Mapping]]] = ..., support: _Optional[_Iterable[str]] = ...) -> None: ...

class ProposedDesign(_message.Message):
    __slots__ = ("folder", "design", "design_yaml", "files", "note", "declared")
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    DESIGN_YAML_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    DECLARED_FIELD_NUMBER: _ClassVar[int]
    folder: str
    design: _project_pb2.Design
    design_yaml: str
    files: _containers.RepeatedScalarFieldContainer[str]
    note: str
    declared: bool
    def __init__(self, folder: _Optional[str] = ..., design: _Optional[_Union[_project_pb2.Design, _Mapping]] = ..., design_yaml: _Optional[str] = ..., files: _Optional[_Iterable[str]] = ..., note: _Optional[str] = ..., declared: _Optional[bool] = ...) -> None: ...

class UnreadFile(_message.Message):
    __slots__ = ("path", "reason")
    PATH_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    path: str
    reason: str
    def __init__(self, path: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...
