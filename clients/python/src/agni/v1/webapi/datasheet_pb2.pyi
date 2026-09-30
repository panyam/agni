from agni.v1.doc import doc_pb2 as _doc_pb2
from agni.v1.param import param_pb2 as _param_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetDocumentRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class GetDocumentResponse(_message.Message):
    __slots__ = ("extracted", "document", "extract_available")
    EXTRACTED_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    extracted: bool
    document: _doc_pb2.Document
    extract_available: bool
    def __init__(self, extracted: _Optional[bool] = ..., document: _Optional[_Union[_doc_pb2.Document, _Mapping]] = ..., extract_available: _Optional[bool] = ...) -> None: ...

class ExtractDocIRRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class ExtractDocIRResponse(_message.Message):
    __slots__ = ("document",)
    DOCUMENT_FIELD_NUMBER: _ClassVar[int]
    document: _doc_pb2.Document
    def __init__(self, document: _Optional[_Union[_doc_pb2.Document, _Mapping]] = ...) -> None: ...

class GetPartSpecRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class GetPartSpecResponse(_message.Message):
    __slots__ = ("found", "spec", "version")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    found: bool
    spec: _param_pb2.PartSpec
    version: str
    def __init__(self, found: _Optional[bool] = ..., spec: _Optional[_Union[_param_pb2.PartSpec, _Mapping]] = ..., version: _Optional[str] = ...) -> None: ...

class SavePartSpecRequest(_message.Message):
    __slots__ = ("spec", "base_version", "uri")
    SPEC_FIELD_NUMBER: _ClassVar[int]
    BASE_VERSION_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    spec: _param_pb2.PartSpec
    base_version: str
    uri: str
    def __init__(self, spec: _Optional[_Union[_param_pb2.PartSpec, _Mapping]] = ..., base_version: _Optional[str] = ..., uri: _Optional[str] = ...) -> None: ...

class SavePartSpecResponse(_message.Message):
    __slots__ = ("version", "problems")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    PROBLEMS_FIELD_NUMBER: _ClassVar[int]
    version: str
    problems: _containers.RepeatedCompositeFieldContainer[ValidationProblem]
    def __init__(self, version: _Optional[str] = ..., problems: _Optional[_Iterable[_Union[ValidationProblem, _Mapping]]] = ...) -> None: ...

class ValidationProblem(_message.Message):
    __slots__ = ("kind", "message")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[ValidationProblem.Kind]
        KIND_STRUCTURAL: _ClassVar[ValidationProblem.Kind]
        KIND_COMPLETENESS: _ClassVar[ValidationProblem.Kind]
    KIND_UNSPECIFIED: ValidationProblem.Kind
    KIND_STRUCTURAL: ValidationProblem.Kind
    KIND_COMPLETENESS: ValidationProblem.Kind
    KIND_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    kind: ValidationProblem.Kind
    message: str
    def __init__(self, kind: _Optional[_Union[ValidationProblem.Kind, str]] = ..., message: _Optional[str] = ...) -> None: ...

class RegionAnnotation(_message.Message):
    __slots__ = ("region_id", "type", "bbox", "page", "kind", "label", "draft_params", "confidence")
    REGION_ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    BBOX_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    DRAFT_PARAMS_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    region_id: str
    type: str
    bbox: _doc_pb2.BBox
    page: int
    kind: str
    label: str
    draft_params: _containers.RepeatedCompositeFieldContainer[_param_pb2.Parameter]
    confidence: float
    def __init__(self, region_id: _Optional[str] = ..., type: _Optional[str] = ..., bbox: _Optional[_Union[_doc_pb2.BBox, _Mapping]] = ..., page: _Optional[int] = ..., kind: _Optional[str] = ..., label: _Optional[str] = ..., draft_params: _Optional[_Iterable[_Union[_param_pb2.Parameter, _Mapping]]] = ..., confidence: _Optional[float] = ...) -> None: ...

class AnnotationSet(_message.Message):
    __slots__ = ("doc_id", "author", "annotations")
    DOC_ID_FIELD_NUMBER: _ClassVar[int]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    doc_id: str
    author: str
    annotations: _containers.RepeatedCompositeFieldContainer[RegionAnnotation]
    def __init__(self, doc_id: _Optional[str] = ..., author: _Optional[str] = ..., annotations: _Optional[_Iterable[_Union[RegionAnnotation, _Mapping]]] = ...) -> None: ...

class GetAnnotationsRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class GetAnnotationsResponse(_message.Message):
    __slots__ = ("sets",)
    SETS_FIELD_NUMBER: _ClassVar[int]
    sets: _containers.RepeatedCompositeFieldContainer[AnnotationSet]
    def __init__(self, sets: _Optional[_Iterable[_Union[AnnotationSet, _Mapping]]] = ...) -> None: ...

class SaveAnnotationsRequest(_message.Message):
    __slots__ = ("set", "uri")
    SET_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    set: AnnotationSet
    uri: str
    def __init__(self, set: _Optional[_Union[AnnotationSet, _Mapping]] = ..., uri: _Optional[str] = ...) -> None: ...

class SaveAnnotationsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
